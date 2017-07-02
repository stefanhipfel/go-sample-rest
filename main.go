package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"

	cfenv "github.com/cloudfoundry-community/go-cfenv"
	jwt "github.com/dgrijalva/jwt-go"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
)

const (
	DEFAULT_PORT = "3000"
)

// jwtCustomClaims are custom claims extending default ones.
type jwtCustomClaims struct {
	Name  string `json:"name"`
	Admin bool   `json:"admin"`
	jwt.StandardClaims
}

func main() {

	db := initDB("gotest")
	if err := migrate(db); err != nil {
		log.Println(err)
		log.Println("Person Table ", err)
	}
	startServer()
}

func initDB(database string) *sql.DB {
	db, err := sql.Open("mysql", "root:root@tcp(localhost:3307)/"+database)
	if err != nil {
		log.Println(err.Error())
		panic(err)
	}

	// make sure connection is available
	err = db.Ping()
	if err != nil {
		log.Println(err.Error())
		panic(err)
	}
	return db
}

func migrate(db *sql.DB) error {
	stmt, err := db.Prepare("CREATE TABLE errormessage (message_id int NOT NULL, time varchar(40), status varchar(40), message varchar(40), reason varchar(40), PRIMARY KEY (message_id));")
	defer db.Close()
	if err != nil {
		log.Println(err.Error())
		return err
	}
	_, err = stmt.Exec()
	if err != nil {
		log.Println(err)
		return err
	}
	log.Println("errormessage Table successfully migrated....")
	return nil
}

func startServer() {
	appEnv, err := cfenv.Current()
	if err != nil {
		log.Println(err)
		log.Println("Warning, cf env not available", appEnv)
	}

	var port string
	if port = os.Getenv("PORT"); len(port) == 0 {
		log.Printf("Warning, PORT not set. Defaulting to %+v", DEFAULT_PORT)
		port = DEFAULT_PORT
	}

	// Configure middleware with the custom claims type
	config := middleware.JWTConfig{
		Claims:     &jwtCustomClaims{},
		SigningKey: []byte("secret"),
	}

	e := echo.New()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.JWTWithConfig(config))
	e.Use(checkClaimsHandler)

	// Routes
	//The server runs each handler in a separate goroutine so that it can serve multiple requests simultaneously
	// Restricted group
	//r := e.Group("/v1")
	e.GET("/v1/ErrorMessages/", errorMessagesHandler)
	e.Logger.Fatal(e.Start(":" + port))
}

// checkClaimsHandler middleware checks jwt claims.
func checkClaimsHandler(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		user := c.Get("user").(*jwt.Token)
		claims := user.Claims.(*jwtCustomClaims)
		admin := claims.Admin
		fmt.Println(admin)
		if admin {
			return next(c)
		}
		return echo.ErrUnauthorized
	}
}

func errorMessagesHandler(c echo.Context) error {
	fromTime := c.QueryParam("from")
	toTime := c.QueryParam("to")

	db := initDB("gotest")
	var status, message, reason, time string
	err := db.QueryRow("SELECT status, message, reason, time FROM errormessage WHERE time = ?", fromTime).Scan(&status, &message, &reason, &time)

	if err != nil {
		log.Println("Error, Query table: ", err)
	}

	respone := ErrorMessage{Time: time, MessageID: "1", Status: status, Message: message, Reason: reason}

	log.Println(fromTime, toTime)
	defer db.Close()

	return c.JSON(http.StatusOK, respone)
}
