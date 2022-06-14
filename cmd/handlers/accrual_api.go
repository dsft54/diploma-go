package handlers

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"diploma/cmd/storage"

	"github.com/go-resty/resty/v2"
)

func accrualRequests(orders []string) ([]*storage.AccrualResponse, error) {
	accrualResults := make([]*storage.AccrualResponse, 0)
	client := resty.New()
	client.SetTimeout(time.Second * 1)
	for _, order := range orders {
		var result *storage.AccrualResponse
		res, err := client.R().Get("http://localhost:8080/api/orders/"+order)
		if err != nil {
			return accrualResults, err
		}
		if res.StatusCode() != 200 {
			accrualResults = append(accrualResults, &storage.AccrualResponse{
				Order: order,
				Status: "INVALID",
				Accrual: 0,
			})
			continue
		}
		err = json.Unmarshal(res.Body(), &result)
		if err != nil {
			return accrualResults, err
		}
		accrualResults = append(accrualResults, result)
	}
	return accrualResults, nil
}

func StartAccrualAPI(ctx context.Context, s *storage.Storage, wg *sync.WaitGroup) {
	collectorTimer := time.NewTicker(time.Second * 2)
	defer wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case <-collectorTimer.C:
			orders, err := s.AccrualProcessingSelector()
			if err != nil {
				log.Println("DB connection not working in accrual handler", err)
			}
			if len(orders) != 0 {
				log.Println("New orders in processing status: ", orders)
				res, err := accrualRequests(orders)
				if err != nil {
					log.Println("Failed to acquire accrual results", err)
				}
				err = s.AccrualUpdateOrders(res)
				if err != nil {
					log.Println("Failed to update tables with new accrual", err)
				}
			}
			orders, err = s.AccrualNewSelector()
			if err != nil {
				log.Println("DB connection not working in accrual handler", err)
			}
			if len(orders) != 0 {
				log.Println("New orders: ", orders)
				res, err := accrualRequests(orders)
				if err != nil {
					log.Println("Failed to acquire accrual results", err)
				}
				err = s.AccrualUpdateOrders(res)
				if err != nil {
					log.Println("Failed to update tables with new accrual", err)
				}
			}
		}
	}
}
