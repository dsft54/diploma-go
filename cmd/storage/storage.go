package storage

import (
	"context"
	"errors"

	"github.com/jackc/pgx"
)

type Storage struct {
	Connection *pgx.Conn
	Context    context.Context
}

func (s *Storage) Ping() error {
	err := s.Connection.Ping(s.Context)
	if err != nil {
		return err
	}
	return nil
}

func NewStorageConnection(ctx context.Context, uri string) (*Storage, error) {
	store := new(Storage)
	if uri == "" {
		return store, errors.New("database connection uri is empty")
	}
	connectionConfig, err := pgx.ParseConnectionString(uri)
	if err != nil {
		return store, err
	}
	store.Connection, err = pgx.Connect(connectionConfig)
	if err != nil {
		return store, err
	}
	store.Context = ctx
	return store, nil
}

func (s *Storage) PrepareWorkingTables() error {
	err := s.prepareUsersTable()
	if err != nil {
		return err
	}
	err = s.prepareOrdersTable()
	if err != nil {
		return err
	}
	err = s.prepareWithdrawalsTable()
	if err != nil {
		return err
	}
	return nil
}

func (s *Storage) prepareUsersTable() error {
	_, err := s.Connection.Exec(
		`CREATE TABLE IF NOT EXISTS users (
			id serial primary key,
			login TEXT UNIQUE,
			password_hash TEXT,
			creation_time TIMESTAMP,
			balance DOUBLE PRECISION,
			withdrawals DOUBLE PRECISION
		);`)
	if err != nil {
		return errors.New("failed to create table users in db")
	}
	return nil
}

func (s *Storage) prepareOrdersTable() error {
	_, err := s.Connection.Exec(
		`CREATE TABLE IF NOT EXISTS orders (
			id serial primary key,
			order_number TEXT UNIQUE,
			owner TEXT REFERENCES users (login) ON DELETE CASCADE,
			creation_time TIMESTAMP,
			status TEXT,
			accrual DOUBLE PRECISION
		);`)
	if err != nil {
		return errors.New("failed to create table orders in db")
	}
	return nil
}

func (s *Storage) prepareWithdrawalsTable() error {
	_, err := s.Connection.Exec(
		`CREATE TABLE IF NOT EXISTS withdrawals (
			id serial primary key,
			order_number TEXT,
			owner TEXT REFERENCES users (login) ON DELETE CASCADE,
			creation_time TIMESTAMP,
			withdraw DOUBLE PRECISION
		);`)
	if err != nil {
		return errors.New("failed to create table withdrawals in db")
	}
	return nil
}

func (s *Storage) FindUserExists(login string) (bool, error) {
	var exists bool
	row := s.Connection.QueryRow(
		`SELECT EXISTS (SELECT 1 FROM users 
			WHERE users.login=$1);`,
		login)
	err := row.Scan(&exists)
	if err != nil {
		return exists, err
	}
	return exists, nil
}

func (s *Storage) FindLoginPass(login, password string) (bool, error) {
	var exists bool
	row := s.Connection.QueryRow(
		`SELECT EXISTS (SELECT 1 FROM users 
			WHERE users.login=$1 AND users.password_hash=$2);`,
		login, password)
	err := row.Scan(&exists)
	if err != nil {
		return exists, err
	}
	return exists, nil
}

func (s *Storage) CreateUser(rf *RegisterForm) error {
	_, err := s.Connection.Exec(
		`INSERT INTO users (login, password_hash, creation_time, balance, withdrawals)
			VALUES ($1, $2, $3, $4, $5);`,
		rf.Login, rf.Password, rf.TimeCreated, 0, 0)
	if err != nil {
		return err
	}
	return nil
}

func (s *Storage) DeleteUser(login string) error {
	_, err := s.Connection.Exec(
		`DELETE FROM users WHERE EXISTS (SELECT 1 FROM users
			WHERE users.login = $1);`,
		login)
	if err != nil {
		return err
	}
	return nil
}

func (s *Storage) CreateOrder(user, order, timeCreated string) error {
	_, err := s.Connection.Exec(
		`INSERT INTO orders (order_number, owner, creation_time, status)
			VALUES ($1, $2, $3, $4);`,
		order, user, timeCreated, "NEW")
	if err != nil {
		return err
	}
	return nil
}

func (s *Storage) CreateWithdrawOrder(user, order, timeCreated string, withdraw float64) (bool, error) {
	var balance float64
	row := s.Connection.QueryRow(
		`SELECT balance FROM users 
				WHERE users.login=$1;`,
		user)
	err := row.Scan(&balance)
	if err != nil {
		return false, err
	}
	if balance < withdraw {
		return false, nil
	}
	_, err = s.Connection.Exec(
		`INSERT INTO withdrawals (order_number, owner, creation_time, withdraw)
			VALUES ($1, $2, $3, $4);`,
		order, user, timeCreated, withdraw)
	if err != nil {
		return false, nil
	}
	_, err = s.Connection.Exec(
		`INSERT INTO users (login, withdrawals)
			VALUES ($1, $2)
			ON CONFLICT (login) DO UPDATE
			SET balance = users.balance - excluded.withdrawals, withdrawals = excluded.withdrawals + users.withdrawals;`,
		user, withdraw)
	if err != nil {
		return false, nil
	}
	return true, nil
}

func (s *Storage) FindOrderNumberExists(order string) (string, error) {
	var username string
	row := s.Connection.QueryRow(
		`SELECT owner FROM orders 
			WHERE EXISTS (SELECT 1 FROM orders WHERE orders.order_number=$1);`,
		order)
	err := row.Scan(&username)
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return "", nil
		default:
			return "", err
		}
	}
	return username, nil
}

func (s *Storage) FindOrdersByOwner(owner string) ([]*OrderStatus, error) {
	orders := make([]*OrderStatus, 0)
	rows, err := s.Connection.Query(
		`SELECT order_number, creation_time, status, accrual FROM orders 
			WHERE orders.owner=$1;`,
		owner)
	if err != nil {
		return orders, err
	}
	for rows.Next() {
		order := new(OrderStatus)
		err = rows.Scan(&order.Number, &order.UploadTime, &order.Status, &order.Accrual)
		if err != nil {
			return orders, err
		}
		orders = append(orders, order)
	}
	err = rows.Err()
	if err != nil {
		return orders, err
	}
	return orders, nil
}

func (s *Storage) AccrualProcessingSelector() ([]string, error) {
	orders := make([]string, 0)
	rows, err := s.Connection.Query(
		`SELECT order_number FROM orders 
			WHERE orders.status=$1;`,
		"PROCESSING")
	if err != nil {
		return orders, err
	}
	for rows.Next() {
		order := ""
		err = rows.Scan(&order)
		if err != nil {
			return orders, err
		}
		orders = append(orders, order)
	}
	err = rows.Err()
	if err != nil {
		return orders, err
	}
	return orders, nil
}

func (s *Storage) AccrualNewSelector() ([]string, error) {
	orders := make([]string, 0)
	rows, err := s.Connection.Query(
		`SELECT order_number FROM orders 
			WHERE orders.status=$1;`,
		"NEW")
	if err != nil {
		return orders, err
	}
	for rows.Next() {
		order := ""
		err = rows.Scan(&order)
		if err != nil {
			return orders, err
		}
		orders = append(orders, order)
	}
	err = rows.Err()
	if err != nil {
		return orders, err
	}
	for _, order := range orders {
		_, err = s.Connection.Exec(
			`INSERT INTO orders (order_number, status)
				VALUES ($1, $2)
				ON CONFLICT (order_number) DO UPDATE
				SET status = excluded.status;`,
			order, "PROCESSING")
		if err != nil {
			return orders, err
		}
	}
	return orders, nil
}

func (s *Storage) AccrualUpdateOrders(responses []*AccrualResponse) error {
	for _, res := range responses {
		var owner string
		row := s.Connection.QueryRow(
			`SELECT owner FROM orders 
				WHERE orders.order_number=$1;`,
			res.Order)
		err := row.Scan(&owner)
		if err != nil {
			return err
		}
		_, err = s.Connection.Exec(
			`INSERT INTO orders (order_number, status, accrual)
				VALUES ($1, $2, $3)
				ON CONFLICT (order_number) DO UPDATE
				SET status = excluded.status, accrual = excluded.accrual;`,
			res.Order, res.Status, res.Accrual)
		if err != nil {
			return err
		}
		_, err = s.Connection.Exec(
			`INSERT INTO users (login, balance)
				VALUES ($1, $2)
				ON CONFLICT (login) DO UPDATE
				SET balance = excluded.balance + users.balance;`,
			owner, res.Accrual)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Storage) GetUserBalance(user string) (*UserBalance, error) {
	var balance UserBalance
	row := s.Connection.QueryRow(
		`SELECT balance, withdrawals FROM users 
				WHERE users.login=$1;`,
		user)
	err := row.Scan(&balance.Current, &balance.Withdrawn)
	if err != nil {
		return &balance, err
	}
	return &balance, nil
}

func (s *Storage) FindWithdrawalsByOwner(owner string) ([]*OrderWithdrawn, error) {
	orders := make([]*OrderWithdrawn, 0)
	rows, err := s.Connection.Query(
		`SELECT order_number, creation_time, withdraw FROM withdrawals 
			WHERE withdrawals.owner=$1;`,
		owner)
	if err != nil {
		return orders, err
	}
	for rows.Next() {
		order := new(OrderWithdrawn)
		err = rows.Scan(&order.Order, &order.ProcessTime, &order.Sum)
		if err != nil {
			return orders, err
		}
		orders = append(orders, order)
	}
	err = rows.Err()
	if err != nil {
		return orders, err
	}
	return orders, nil
}
