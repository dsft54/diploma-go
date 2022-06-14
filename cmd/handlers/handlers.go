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

// Хендлер: POST /api/user/register.
// Регистрация производится по паре логин/пароль. Каждый логин должен быть уникальным. После успешной регистрации должна происходить автоматическая аутентификация пользователя.
// Формат запроса:
// POST /api/user/register HTTP/1.1
// Content-Type: application/json
// ...

// {
// 	"login": "<login>",
// 	"password": "<password>"
// }
// Возможные коды ответа:
// 200 — пользователь успешно зарегистрирован и аутентифицирован;
// 400 — неверный формат запроса;
// 409 — логин уже занят;
// 500 — внутренняя ошибка сервера.
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

// Хендлер: POST /api/user/login.
// Аутентификация производится по паре логин/пароль.
// Формат запроса:
// POST /api/user/login HTTP/1.1
// Content-Type: application/json
// ...

// {
//     "login": "<login>",
//     "password": "<password>"
// }
// Возможные коды ответа:
// 200 — пользователь успешно аутентифицирован;
// 400 — неверный формат запроса;
// 401 — неверная пара логин/пароль;
// 500 — внутренняя ошибка сервера.
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

// Хендлер доступен только аутентифицированным пользователям. Номером заказа является последовательность цифр произвольной длины.
// Номер заказа может быть проверен на корректность ввода с помощью алгоритма Луна.
// Формат запроса:
// POST /api/user/orders HTTP/1.1
// Content-Type: text/plain
// ...

// 12345678903
// Возможные коды ответа:
// 200 — номер заказа уже был загружен этим пользователем;
// 202 — новый номер заказа принят в обработку;
// 400 — неверный формат запроса;
// 401 — пользователь не аутентифицирован;
// 409 — номер заказа уже был загружен другим пользователем;
// 422 — неверный формат номера заказа;
// 500 — внутренняя ошибка сервера.
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
		luhnChecksum := func(number int) int {
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
		luhnValid := func(number int) bool {
			return (number%10+luhnChecksum(number/10))%10 == 0
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

// Хендлер: GET /api/user/orders.
// Хендлер доступен только авторизованному пользователю. Номера заказа в выдаче должны быть отсортированы по времени загрузки от самых старых к самым новым. Формат даты — RFC3339.
// Доступные статусы обработки расчётов:
// NEW — заказ загружен в систему, но не попал в обработку;
// PROCESSING — вознаграждение за заказ рассчитывается;
// INVALID — система расчёта вознаграждений отказала в расчёте;
// PROCESSED — данные по заказу проверены и информация о расчёте успешно получена.
// Формат запроса:
// GET /api/user/orders HTTP/1.1
// Content-Length: 0
// Возможные коды ответа:
// 200 — успешная обработка запроса.
// Формат ответа:
//   200 OK HTTP/1.1
//   Content-Type: application/json
//   ...

//   [
//       {
//           "number": "9278923470",
//           "status": "PROCESSED",
//           "accrual": 500,
//           "uploaded_at": "2020-12-10T15:15:45+03:00"
//       },
//       {
//           "number": "12345678903",
//           "status": "PROCESSING",
//           "uploaded_at": "2020-12-10T15:12:01+03:00"
//       },
//       {
//           "number": "346436439",
//           "status": "INVALID",
//           "uploaded_at": "2020-12-09T16:09:53+03:00"
//       }
//   ]

// 204 — нет данных для ответа.
// 401 — пользователь не авторизован.
// 500 — внутренняя ошибка сервера.
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

// Хендлер: GET /api/user/balance.
// Хендлер доступен только авторизованному пользователю. В ответе должны содержаться данные о текущей сумме баллов лояльности, а также сумме использованных за весь период регистрации баллов.
// Формат запроса:
// GET /api/user/balance HTTP/1.1
// Content-Length: 0 
// Возможные коды ответа:
// 200 — успешная обработка запроса.
// Формат ответа:
//   200 OK HTTP/1.1
//   Content-Type: application/json
//   ...
  
//   {
//       "current": 500.5,
//       "withdrawn": 42
//   }
   
// 401 — пользователь не авторизован.
// 500 — внутренняя ошибка сервера.
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

// Хендлер: POST /api/user/balance/withdraw
// Хендлер доступен только авторизованному пользователю. Номер заказа представляет собой гипотетический номер нового заказа пользователя, в счёт оплаты которого списываются баллы.
// Примечание: для успешного списания достаточно успешной регистрации запроса, никаких внешних систем начисления не предусмотрено и не требуется реализовывать.
// Формат запроса:
// POST /api/user/balance/withdraw HTTP/1.1
// Content-Type: application/json

// {
//     "order": "2377225624",
//     "sum": 751
// } 
// Здесь order — номер заказа, а sum — сумма баллов к списанию в счёт оплаты.
// Возможные коды ответа:
// 200 — успешная обработка запроса;
// 401 — пользователь не авторизован;
// 402 — на счету недостаточно средств;
// 422 — неверный номер заказа;
// 500 — внутренняя ошибка сервера.
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
		luhnChecksum := func(number int) int {
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
		luhnValid := func(number int) bool {
			return (number%10+luhnChecksum(number/10))%10 == 0
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

// Хендлер: GET /api/user/balance/withdrawals.
// Хендлер доступен только авторизованному пользователю. Факты выводов в выдаче должны быть отсортированы по времени вывода от самых старых к самым новым. Формат даты — RFC3339.
// Формат запроса:
// GET /api/user/withdrawals HTTP/1.1
// Content-Length: 0 
// Возможные коды ответа:
// 200 — успешная обработка запроса.
// Формат ответа:
//   200 OK HTTP/1.1
//   Content-Type: application/json
//   ...
  
//   [
//       {
//           "order": "2377225624",
//           "sum": 500,
//           "processed_at": "2020-12-09T16:09:57+03:00"
//       }
//   ]
   
// 204 — нет ни одного списания.
// 401 — пользователь не авторизован.
// 500 — внутренняя ошибка сервера.
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