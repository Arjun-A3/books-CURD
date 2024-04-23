package main

import (
	"context"
	"net/http"

	"my-gin-project/configs"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

var DB *mongo.Client

func init() {
	DB = configs.DB
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
	r.DELETE("/clear-mongodb", clearMongoDB)

	r.Run(":8080")
}

func getBooks(c *gin.Context) {
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

	c.JSON(http.StatusOK, bookList)
}

func getBook(c *gin.Context) {
	id := c.Param("id")
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid book ID"})
		return
	}

	collection := configs.GetCollection(DB, "books")
	var book Book
	err = collection.FindOne(context.Background(), bson.M{"_id": objectID}).Decode(&book)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Book not found"})
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

	c.JSON(http.StatusOK, gin.H{"message": "Book deleted"})
}

func clearMongoDB(c *gin.Context) {
	collection := configs.GetCollection(DB, "books")
	_, err := collection.DeleteMany(context.Background(), bson.M{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to clear MongoDB storage"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "MongoDB storage cleared"})
}
