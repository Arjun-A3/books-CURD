package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

var rdb *redis.Client

func init() {
	rdb = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})
}

type Book struct {
	ID     int    `json:"id"`
	Title  string `json:"title"`
	Author string `json:"author"`
}

var books []Book

func main() {
	r := gin.Default()

	// Routes
	r.GET("/books", getBooks)
	r.GET("/books/:id", getBook)
	r.POST("/books", createBook)
	r.PUT("/books/:id", updateBook)
	r.DELETE("/books/:id", deleteBook)
	r.DELETE("/clear-redis", clearRedis)

	r.Run(":8080")
}

func getBooks(c *gin.Context) {

	keys, err := rdb.Keys(context.Background(), "book:*").Result()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get books from Redis"})
		return
	}

	var bookList []Book
	for _, key := range keys {
		bookJSON, err := rdb.HGetAll(context.Background(), key).Result()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get book data from Redis"})
			return
		}

		var book Book
		if err := json.Unmarshal([]byte(bookJSON["data"]), &book); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to unmarshal book data"})
			return
		}

		bookList = append(bookList, book)
	}

	c.JSON(http.StatusOK, bookList)
}

func getBook(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	bookJSON, err := rdb.HGet(context.Background(), "book:"+strconv.Itoa(id), "data").Result()
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Book not found"})
		return
	}

	var book Book
	if err := json.Unmarshal([]byte(bookJSON), &book); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to unmarshal book data"})
		return
	}

	c.JSON(http.StatusOK, book)
}

var nextBookIDKey = "next_book_id"

func createBook(c *gin.Context) {
	var newBook Book
	if err := c.ShouldBindJSON(&newBook); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	id, err := rdb.Incr(context.Background(), nextBookIDKey).Result()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate book ID"})
		return
	}
	newBook.ID = int(id)
	bookJSON, err := json.Marshal(newBook)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to marshal book data"})
		return
	}
	if err := rdb.HSet(context.Background(), "book:"+strconv.Itoa(newBook.ID), "data", string(bookJSON)).Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save book to Redis"})
		return
	}
	c.JSON(http.StatusCreated, newBook)
}

func updateBook(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var updatedBook Book
	if err := c.ShouldBindJSON(&updatedBook); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if book exists
	_, err := rdb.HGet(context.Background(), "book:"+strconv.Itoa(id), "data").Result()
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Book not found"})
		return
	}

	//fmt.Println("ID:", strconv.Itoa(id))

	// Update book
	updatedBook.ID = id
	bookJSON, err := json.Marshal(updatedBook)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to marshal book data"})
		return
	}
	if err := rdb.HSet(context.Background(), "book:"+strconv.Itoa(id), "data", string(bookJSON)).Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update book in Redis"})
		return
	}

	c.JSON(http.StatusOK, updatedBook)
}

func deleteBook(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))

	// Check if book exists
	_, err := rdb.HGet(context.Background(), "book:"+strconv.Itoa(id), "data").Result()
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Book not found"})
		return
	}

	// Delete book
	if err := rdb.Del(context.Background(), "book:"+strconv.Itoa(id)).Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete book from Redis"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Book deleted"})
}

func clearRedis(c *gin.Context) {
	// Flush the Redis database
	if err := rdb.FlushDB(context.Background()).Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to clear Redis storage"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Redis storage cleared"})
}
