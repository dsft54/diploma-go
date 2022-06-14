package storage

import "time"

type RegisterForm struct {
	Login       string `json:"login"`
	Password    string `json:"password"`
	TimeCreated string `json:"time_created,omitempty"`
}

type OrderStatus struct {
	Number     string    `json:"number"`
	Status     string    `json:"status"`
	Accrual    *float64  `json:"accrual,omitempty"`
	UploadTime time.Time `json:"uploaded_at"`
}

type AccrualResponse struct {
	Order   string  `json:"order"`
	Status  string  `json:"status"`
	Accrual float64 `json:"accrual"`
}

type UserBalance struct {
	Current   float64 `json:"current"`
	Withdrawn float64 `json:"withdrawn"`
}

type OrderWithdrawn struct {
	Order       string    `json:"order"`
	Sum         float64   `json:"sum"`
	ProcessTime time.Time `json:"processed_at,omitempty"`
}
