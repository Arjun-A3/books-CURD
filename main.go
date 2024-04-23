package main

import (
	"context"
	"encoding/json"
	"net/http"

	"my-gin-project/configs"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

var (
	DB  *mongo.Client
	rdb *redis.Client
)

func init() {
	DB = configs.DB
	rdb = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "", // no password set
		DB:       0,  // use default DB
	})
}

type Book struct {
	ID     primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
	Title  string             `json:"title"`
	Author string             `json:"author"`
}

func main() {
	r := gin.Default()

	// Routes
	r.GET("/books", getBooks)
	r.GET("/books/:id", getBook)
	r.POST("/books", createBook)
	r.PUT("/books/:id", updateBook)
	r.DELETE("/books/:id", deleteBook)

	r.Run(":8080")
}

func getBooks(c *gin.Context) {
	// Check Redis cache first
	bookListStr, err := rdb.Get(context.Background(), "bookList").Result()
	if err == nil {
		// If data exists in cache, return cached data
		var bookList []Book
		if err := json.Unmarshal([]byte(bookListStr), &bookList); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to unmarshal book data"})
			return
		}
		c.JSON(http.StatusOK, bookList)
		return
	}

	// If data is not in Redis cache, query MongoDB
	collection := configs.GetCollection(DB, "books")
	cursor, err := collection.Find(context.Background(), bson.M{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get books from MongoDB"})
		return
	}
	defer cursor.Close(context.Background())

	var bookList []Book
	for cursor.Next(context.Background()) {
		var book Book
		if err := cursor.Decode(&book); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decode book data"})
			return
		}
		bookList = append(bookList, book)
	}

	// Cache the book list in Redis
	bookListJSON, err := json.Marshal(bookList)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to marshal book data"})
		return
	}
	err = rdb.Set(context.Background(), "bookList", bookListJSON, 0).Err()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to cache book data in Redis"})
		return
	}

	c.JSON(http.StatusOK, bookList)
}

func getBook(c *gin.Context) {
	id := c.Param("id")
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid book ID"})
		return
	}

	// Check Redis cache first
	bookStr, err := rdb.Get(context.Background(), "book:"+id).Result()
	if err == nil {
		// If data exists in cache, return cached data
		var book Book
		if err := json.Unmarshal([]byte(bookStr), &book); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to unmarshal book data"})
			return
		}
		c.JSON(http.StatusOK, book)
		return
	}

	// If data is not in Redis cache, query MongoDB
	collection := configs.GetCollection(DB, "books")
	var book Book
	err = collection.FindOne(context.Background(), bson.M{"_id": objectID}).Decode(&book)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Book not found"})
		return
	}

	// Cache the book data in Redis
	bookJSON, err := json.Marshal(book)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to marshal book data"})
		return
	}
	err = rdb.Set(context.Background(), "book:"+id, bookJSON, 0).Err()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to cache book data in Redis"})
		return
	}

	c.JSON(http.StatusOK, book)
}

func createBook(c *gin.Context) {
	var newBook Book
	if err := c.ShouldBindJSON(&newBook); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	collection := configs.GetCollection(DB, "books")
	res, err := collection.InsertOne(context.Background(), newBook)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save book to MongoDB"})
		return
	}
	newBook.ID = res.InsertedID.(primitive.ObjectID)

	// Cache the newly created book data in Redis
	bookJSON, err := json.Marshal(newBook)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to marshal book data"})
		return
	}
	err = rdb.Set(context.Background(), "book:"+newBook.ID.Hex(), bookJSON, 0).Err()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to cache book data in Redis"})
		return
	}

	c.JSON(http.StatusCreated, newBook)
}

func updateBook(c *gin.Context) {
	id := c.Param("id")
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid book ID"})
		return
	}

	var updatedBook Book
	if err := c.ShouldBindJSON(&updatedBook); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	updatedBook.ID = objectID

	collection := configs.GetCollection(DB, "books")
	_, err = collection.UpdateOne(context.Background(), bson.M{"_id": objectID}, bson.M{"$set": updatedBook})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update book in MongoDB"})
		return
	}

	// Update the cached book data in Redis
	updatedBookJSON, err := json.Marshal(updatedBook)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to marshal updated book data"})
		return
	}
	err = rdb.Set(context.Background(), "book:"+id, updatedBookJSON, 0).Err()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update book data in Redis"})
		return
	}

	c.JSON(http.StatusOK, updatedBook)
}

func deleteBook(c *gin.Context) {
	id := c.Param("id")
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid book ID"})
		return
	}

	collection := configs.GetCollection(DB, "books")
	res, err := collection.DeleteOne(context.Background(), bson.M{"_id": objectID})
	if err != nil || res.DeletedCount == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Book not found"})
		return
	}

	// Delete the cached book data from Redis
	err = rdb.Del(context.Background(), "book:"+id).Err()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete book data from Redis"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Book deleted"})
}
