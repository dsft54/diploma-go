package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"diploma/cmd/storage"

	"github.com/gin-gonic/gin"
)

func PingDB(s *storage.Storage) gin.HandlerFunc {
	return func(c *gin.Context) {
		if s.Connection == nil {
			c.Status(http.StatusInternalServerError)
			return
		}
		err := s.Ping()
		if err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusOK)
	}
}

func getUser(cs *storage.CookieStorage, cookies []*http.Cookie) string {
	username := ""
	for _, v := range cookies {
		username = cs.GetUserbyCookie(v.Value)
		if username != "" {
			break
		}
	}
	return username
}

// POST /api/user/register.
func Register(s *storage.Storage, cs *storage.CookieStorage) gin.HandlerFunc {
	return func(c *gin.Context) {
		rawData, err := c.GetRawData()
		if err != nil {
			log.Println(err)
			c.Status(http.StatusInternalServerError)
			return
		}
		registerRequest := &storage.RegisterForm{}
		err = json.Unmarshal(rawData, registerRequest)
		if err != nil {
			log.Println(err)
			c.Status(http.StatusBadRequest)
			return
		}
		userLoginExists, err := s.FindUserExists(registerRequest.Login)
		if err != nil {
			log.Println(err, "Check if user exists failed")
			c.Status(http.StatusInternalServerError)
			return
		}
		if userLoginExists {
			log.Println(err)
			c.Status(http.StatusConflict)
			return
		}
		h := sha256.New()
		h.Write([]byte(registerRequest.Password))
		registerRequest.Password = hex.EncodeToString(h.Sum(nil))
		registerRequest.TimeCreated = time.Now().Format(time.RFC3339)
		err = s.CreateUser(registerRequest)
		if err != nil {
			log.Println(err, "Failed to create new user")
			c.Status(http.StatusInternalServerError)
			return
		}
		// Auth
		h.Write([]byte(registerRequest.Login + cs.RSeed + "cookie"))
		c.SetCookie("gomart_auth", hex.EncodeToString(h.Sum(nil)), 3600, "/", "localhost", false, true)
		cs.AddCookie(&http.Cookie{
			Name:   registerRequest.Login,
			Value:  hex.EncodeToString(h.Sum(nil)),
			MaxAge: 3600,
		})
		c.Status(200)
	}
}

// POST /api/user/login.
func Login(s *storage.Storage, cs *storage.CookieStorage) gin.HandlerFunc {
	return func(c *gin.Context) {
		rawData, err := c.GetRawData()
		if err != nil {
			log.Println(err)
			c.Status(http.StatusInternalServerError)
			return
		}
		loginRequest := &storage.RegisterForm{}
		err = json.Unmarshal(rawData, loginRequest)
		if err != nil {
			log.Println(err)
			c.Status(http.StatusBadRequest)
			return
		}
		h := sha256.New()
		h.Write([]byte(loginRequest.Password))
		userPasswordExists, err := s.FindLoginPass(loginRequest.Login, hex.EncodeToString(h.Sum(nil)))
		if err != nil {
			log.Println(err)
			c.Status(http.StatusInternalServerError)
			return
		}
		if !userPasswordExists {
			c.Status(http.StatusUnauthorized)
			return
		}
		h.Write([]byte(loginRequest.Login + cs.RSeed + "cookie"))
		c.SetCookie("gomart_auth", hex.EncodeToString(h.Sum(nil)), 3600, "/", "localhost", false, true)
		cs.AddCookie(&http.Cookie{
			Name:   loginRequest.Login,
			Value:  hex.EncodeToString(h.Sum(nil)),
			MaxAge: 3600,
		})
		c.Status(http.StatusOK)
	}
}

// GET /api/user/orders.
func GetOrders(s *storage.Storage, cs *storage.CookieStorage) gin.HandlerFunc {
	return func(c *gin.Context) {
		username := getUser(cs, c.Request.Cookies())
		orders, err := s.FindOrdersByOwner(username)
		if err != nil {
			log.Println(err)
			c.Status(http.StatusInternalServerError)
			return
		}
		if len(orders) == 0 {
			c.Status(http.StatusNoContent)
			return
		}
		sort.Slice(orders, func(i, j int) bool {
			return orders[i].UploadTime.Before(orders[j].UploadTime)
		})
		c.JSON(200, orders)
	}
}

// GET /api/user/balance.
func GetBalance(s *storage.Storage, cs *storage.CookieStorage) gin.HandlerFunc {
	return func(c *gin.Context) {
		username := getUser(cs, c.Request.Cookies())
		balance, err := s.GetUserBalance(username)
		if err != nil {
			log.Println(err)
			c.Status(http.StatusInternalServerError)
			return
		}
		c.JSON(200, balance)
	}
}

// GET /api/user/balance/withdrawals.
func GetWithdrawals(s *storage.Storage, cs *storage.CookieStorage) gin.HandlerFunc {
	return func(c *gin.Context) {
		username := getUser(cs, c.Request.Cookies())
		orders, err := s.FindWithdrawalsByOwner(username)
		if err != nil {
			log.Println(err)
			c.Status(http.StatusInternalServerError)
			return
		}
		if len(orders) == 0 {
			c.Status(http.StatusNoContent)
			return
		}
		sort.Slice(orders, func(i, j int) bool {
			return orders[i].ProcessTime.Before(orders[j].ProcessTime)
		})
		c.JSON(200, orders)
	}
}

func luhnChecksum(number int) int {
	var luhn int
	for i := 0; number > 0; i++ {
		cur := number % 10
		if i%2 == 0 {
			cur = cur * 2
			if cur > 9 {
				cur = cur%10 + cur/10
			}
		}
		luhn += cur
		number = number / 10
	}
	return luhn % 10
}
func luhnValid(number int) bool {
	return (number%10+luhnChecksum(number/10))%10 == 0
}

// POST /api/user/orders
func PlaceOrder(s *storage.Storage, cs *storage.CookieStorage) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !strings.Contains(c.Request.Header.Get("Content-Type"), "text") {
			c.Status(http.StatusBadRequest)
			return
		}
		rawNumber, err := c.GetRawData()
		if err != nil {
			log.Println(err)
			c.Status(http.StatusInternalServerError)
			return
		}
		orderNumber, err := strconv.Atoi(string(rawNumber))
		if err != nil {
			log.Println(err)
			c.Status(http.StatusInternalServerError)
			return
		}
		if !luhnValid(orderNumber) {
			c.Status(http.StatusUnprocessableEntity)
			return
		}
		username := getUser(cs, c.Request.Cookies())
		ordUser, err := s.FindOrderNumberExists(string(rawNumber))
		if err != nil {
			log.Println(err)
			c.Status(http.StatusInternalServerError)
			return
		}
		creationTime := time.Now().Format(time.RFC3339)
		if ordUser == "" {
			err = s.CreateOrder(username, string(rawNumber), creationTime)
			if err != nil {
				log.Println(err)
				c.Status(http.StatusInternalServerError)
				return
			}
			c.Status(http.StatusAccepted)
			return
		}
		if ordUser != username {
			c.Status(http.StatusConflict)
			return
		}
		c.Status(http.StatusOK)
	}
}

// POST /api/user/balance/withdraw
func PlaceWithdrawOrder(s *storage.Storage, cs *storage.CookieStorage) gin.HandlerFunc {
	return func(c *gin.Context) {
		var wOrder storage.OrderWithdrawn
		rawData, err := c.GetRawData()
		if err != nil {
			log.Println(err)
			c.Status(http.StatusInternalServerError)
			return
		}
		err = json.Unmarshal(rawData, &wOrder)
		if err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
		orderNumber, err := strconv.Atoi(wOrder.Order)
		if err != nil {
			log.Println(err)
			c.Status(http.StatusInternalServerError)
			return
		}
		if !luhnValid(orderNumber) {
			c.Status(http.StatusUnprocessableEntity)
			return
		}
		username := getUser(cs, c.Request.Cookies())
		creationTime := time.Now().Format(time.RFC3339)
		enough, err := s.CreateWithdrawOrder(username, wOrder.Order, creationTime, wOrder.Sum)
		if err != nil {
			log.Println(err)
			c.Status(http.StatusInternalServerError)
			return
		}
		if !enough {
			c.Status(http.StatusPaymentRequired)
			return
		}
		c.Status(http.StatusOK)
	}
}