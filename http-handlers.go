package main

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
	"io/ioutil"
	"math"
	"net/http"
	"strconv"
	"text/template"
	"time"
)

func handleSaveBook(w http.ResponseWriter, r *http.Request, log *logrus.Logger) {
	log.WithFields(logrus.Fields{
		"method": r.Method,
		"path":   r.URL.Path,
	}).Info("Handling save book request")

	var id = 0
	var err error

	r.ParseForm()
	params := r.PostForm
	idStr := params.Get("id")

	if len(idStr) > 0 {
		id, err = strconv.Atoi(idStr)
		if err != nil {
			log.WithError(err).Error("Failed to convert ID to integer")
			renderErrorPage(w, err)
			return
		}
	}

	name := params.Get("name")
	author := params.Get("author")

	pagesStr := params.Get("pages")
	pages := 0
	if len(pagesStr) > 0 {
		pages, err = strconv.Atoi(pagesStr)
		if err != nil {
			log.WithError(err).Error("Failed to convert pages to integer")
			renderErrorPage(w, err)
			return
		}
	}

	publicationDateStr := params.Get("publicationDate")
	var publicationDate time.Time

	if len(publicationDateStr) > 0 {
		publicationDate, err = time.Parse("2006-01-02", publicationDateStr)
		if err != nil {
			log.WithError(err).Error("Failed to parse publication date")
			renderErrorPage(w, err)
			return
		}
	}

	if id == 0 {
		_, err = insertBook(name, author, pages, publicationDate)
	} else {
		_, err = updateBook(id, name, author, pages, publicationDate)
	}

	if err != nil {
		log.WithError(err).Error("Failed to save book")
		renderErrorPage(w, err)
		return
	}

	log.Info("Book saved successfully")
	http.Redirect(w, r, "/", 302)
}

func handleListBooks(w http.ResponseWriter, r *http.Request, log *logrus.Logger) {
	log.Info("Handling list books request")

	books, err := allBooksAdmin()
	if err != nil {
		log.Errorf("Error retrieving books: %v", err)
		renderErrorPage(w, err)
		return
	}

	buf, err := ioutil.ReadFile("www/index.html")
	if err != nil {
		log.Errorf("Error reading HTML file: %v", err)
		renderErrorPage(w, err)
		return
	}

	var page = IndexPage{AllBooks: books}
	indexPage := string(buf)
	t := template.Must(template.New("indexPage").Parse(indexPage))
	err = t.Execute(w, page)
	if err != nil {
		log.Errorf("Error executing template: %v", err)
		renderErrorPage(w, err)
		return
	}

	log.Info("List books request handled successfully")
}

func handleViewBook(w http.ResponseWriter, r *http.Request, log *logrus.Logger) {
	params := r.URL.Query()
	idStr := params.Get("id")

	var currentBook = Book{}
	currentBook.PublicationDate = time.Now()

	if len(idStr) > 0 {
		id, err := strconv.Atoi(idStr)
		if err != nil {
			log.Error("Failed to convert ID to integer:", err)
			renderErrorPage(w, err)
			return
		}

		currentBook, err = getBook(id)
		if err != nil {
			log.Error("Failed to retrieve book from the database:", err)
			renderErrorPage(w, err)
			return
		}
	}

	buf, err := ioutil.ReadFile("www/book.html")
	if err != nil {
		log.Error("Failed to read book.html file:", err)
		renderErrorPage(w, err)
		return
	}

	var page = BookPage{TargetBook: currentBook}
	bookPage := string(buf)
	t := template.Must(template.New("bookPage").Parse(bookPage))
	err = t.Execute(w, page)
	if err != nil {
		log.Error("Failed to execute template:", err)
		renderErrorPage(w, err)
		return
	}
}

func handleDeleteBook(w http.ResponseWriter, r *http.Request, log *logrus.Logger) {
	log.WithFields(logrus.Fields{
		"method": r.Method,
		"path":   r.URL.Path,
	}).Info("Handling delete book request")

	params := r.URL.Query()
	idStr := params.Get("id")

	if len(idStr) > 0 {
		id, err := strconv.Atoi(idStr)
		if err != nil {
			log.WithError(err).Error("Failed to convert ID to integer")
			renderErrorPage(w, err)
			return
		}

		n, err := removeBook(id)
		if err != nil {
			log.WithError(err).Error("Failed to remove book")
			renderErrorPage(w, err)
			return
		}

		log.WithField("rowsRemoved", n).Info("Book removed successfully")
	}

	http.Redirect(w, r, "/", 302)
}

func renderErrorPage(w http.ResponseWriter, errorMsg error) {
	buf, err := ioutil.ReadFile("www/error.html")
	if err != nil {
		log.Printf("%v\n", err)
		fmt.Fprintf(w, "%v\n", err)
		return
	}

	var page = ErrorPage{ErrorMsg: errorMsg.Error()}
	errorPage := string(buf)
	t := template.Must(template.New("errorPage").Parse(errorPage))
	t.Execute(w, page)
}

func handleListProducts(w http.ResponseWriter, r *http.Request, log *logrus.Logger) {
	log.WithFields(logrus.Fields{
		"method": r.Method,
		"path":   r.URL.Path,
	}).Info("Handling list products request")

	// Retrieve filtering, sorting, and pagination parameters from the URL
	genre := r.URL.Query().Get("genre")
	onSale := r.URL.Query().Get("onSale")
	priceFrom := r.URL.Query().Get("priceFrom")
	priceTo := r.URL.Query().Get("priceTo")
	sortBy := r.URL.Query().Get("sortBy")
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")
	page := r.URL.Query().Get("page")

	limit := 8
	offset := 0

	if limitStr != "" {
		limit, _ = strconv.Atoi(limitStr)
	}
	if offsetStr != "" {
		offset, _ = strconv.Atoi(offsetStr)
	}

	if p, err := strconv.Atoi(page); err == nil && p > 1 {
		offset = (p - 1) * limit
	}

	query := "SELECT * FROM books"
	whereClause := ""
	// Add filters
	if genre != "" {
		whereClause += " genre = '" + genre + "' AND"
	}
	if onSale != "" {
		whereClause += " onSale = '" + onSale + "' AND"
	}
	if priceFrom != "" && priceTo != "" {
		whereClause += " price BETWEEN " + priceFrom + " AND " + priceTo + " AND"
	}
	// Remove trailing " AND" if it exists
	if len(whereClause) > 0 && whereClause[len(whereClause)-4:] == " AND" {
		whereClause = whereClause[:len(whereClause)-4]
	}

	// Add where clause to the query if there's any filter
	if whereClause != "" {
		query += " WHERE" + whereClause
	}

	// Add sorting
	if sortBy != "" {
		switch sortBy {
		case "priceASC":
			query += " ORDER BY price"
		case "priceDESC":
			query += " ORDER BY price DESC"
		case "alphaASC":
			query += " ORDER BY name"
		case "alphaDESC":
			query += " ORDER BY name DESC"
		default:
			query += ""
		}
	}

	books, err := getAllBooks(query, limit, offset)
	totalBooks, _ := getTotalBooksCount()
	totalPages := int(math.Ceil(float64(totalBooks) / float64(limit)))

	prevOffset := offset - limit
	nextOffset := offset + limit

	var pages []Page
	for i := 1; i <= totalPages; i++ {
		pages = append(pages, Page{
			PageNumber: i,
			Offset:     (i - 1) * limit,
		})
	}

	data := struct {
		Books      []Book
		Pages      []Page
		PrevOffset int
		NextOffset int
	}{
		Books:      books,
		Pages:      pages,
		PrevOffset: prevOffset,
		NextOffset: nextOffset,
	}

	tmpl, err := template.ParseFiles("www/products.html")
	if err != nil {
		log.WithError(err).Error("Failed to parse products template file")
		renderErrorPage(w, err)
		return
	}

	err = tmpl.Execute(w, data)
	if err != nil {
		log.WithError(err).Error("Failed to execute products template")
		renderErrorPage(w, err)
		return
	}

	log.Info("List products request handled successfully")
}

func signupHandler(w http.ResponseWriter, r *http.Request, log *logrus.Logger) {
	err := r.ParseForm()
	if err != nil {
		log.WithFields(logrus.Fields{
			"error": err.Error(),
		}).Error("Invalid request")
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")

	if username != "" && password != "" {
		// Hash the password before storing it in the database
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			log.WithFields(logrus.Fields{
				"error": err.Error(),
			}).Error("Failed to hash password")
			http.Error(w, "Failed to hash password", http.StatusInternalServerError)
			return
		}

		// Insert user into the database
		_, err = db.Exec("INSERT INTO users (username, password_hash) VALUES ($1, $2)", username, string(hashedPassword))
		if err != nil {
			log.WithFields(logrus.Fields{
				"error": err.Error(),
			}).Error("Failed to create user")
			http.Error(w, "Failed to create user", http.StatusInternalServerError)
			return
		}

		// Successful signup, redirect to login page
		log.WithFields(logrus.Fields{
			"username": username,
		}).Info("User created successfully")
		http.Redirect(w, r, "/products", http.StatusFound)
	}
	http.ServeFile(w, r, "www/sign-up.html")
}

func loginHandler(w http.ResponseWriter, r *http.Request, log *logrus.Logger) {
	loginSuccess := false

	err := r.ParseForm()
	if err != nil {
		log.WithFields(logrus.Fields{
			"error": err.Error(),
		}).Error("Invalid request")
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")

	if username != "" {
		var storedPassword string
		err = db.QueryRow("SELECT password_hash FROM users WHERE username = $1", username).Scan(&storedPassword)
		if err != nil {
			log.WithFields(logrus.Fields{
				"error":    err.Error(),
				"username": username,
			}).Error("Invalid username or password")
			http.Error(w, "Invalid username or password", http.StatusUnauthorized)
			return
		}

		if storedPassword != password {
			log.WithFields(logrus.Fields{
				"username": username,
			}).Warn("Invalid password")
			http.Error(w, "Invalid username or password", http.StatusUnauthorized)
			return
		}

		loginSuccess = true
	}

	if loginSuccess {
		log.WithField("username", username).Info("User logged in successfully")
		http.Redirect(w, r, "/products", http.StatusFound)
	} else {
		log.Warn("Login attempt failed")
		http.ServeFile(w, r, "www/login.html")
	}
}
