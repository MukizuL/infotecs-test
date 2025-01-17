package storage

import (
	"context"
	"database/sql"
	"errors"
	"github.com/google/uuid"
	"log"
)

var ErrNotEnoughMoney = errors.New("недостаточно средств")
var ErrWrongSender = errors.New("нет такого отправителя")
var ErrWrongReceiver = errors.New("нет такого получателя")
var ErrPurseNotFound = errors.New("нет такого кошелька")

type Purse struct {
	Id      uuid.UUID
	Balance float64
}

type Transaction struct {
	Id       int     `json:"id"`
	Sender   string  `json:"sender"`
	Receiver string  `json:"receiver"`
	Amount   float64 `json:"amount"`
	Time     string  `json:"time"`
}

type StorageConn struct {
	DB *sql.DB
}

func (s *StorageConn) Init() {
	purses, transactions := false, false
	stmt := `SELECT table_name
FROM information_schema.tables
WHERE table_name = 'purses'`

	rows, err := s.DB.Query(stmt)
	if err != nil {
		log.Fatal(err.Error())
	}

	if rows.Next() {
		purses = true
	}

	rows.Close()

	stmt = `SELECT table_name
FROM information_schema.tables
WHERE table_name = 'transactions'`

	rows, err = s.DB.Query(stmt)
	if err != nil {
		log.Fatal(err.Error())
	}

	if rows.Next() {
		transactions = true
	}

	rows.Close()

	tx, err := s.DB.BeginTx(context.Background(), nil)
	if err != nil {
		log.Fatal(err.Error())
	}

	if !purses {
		stmt = `CREATE TABLE purses (
    id UUID PRIMARY KEY,
    balance INT
)`
		_, err = tx.Exec(stmt)
		if err != nil {
			tx.Rollback()
			log.Fatal(err.Error())
		}

		for i := 0; i < 10; i++ {
			stmt = `INSERT INTO purses (id, balance) VALUES ($1, $2)`

			_, err := tx.Exec(stmt, uuid.New(), 10000)
			if err != nil {
				tx.Rollback()
				log.Fatal(err.Error())
			}
		}
	}

	if !transactions {
		stmt = `
CREATE TABLE transactions (
    id SERIAL PRIMARY KEY,
    sender UUID,
    receiver UUID,
    amount INT,
    time TIMESTAMP
)`
		_, err = tx.Exec(stmt)
		if err != nil {
			tx.Rollback()
			log.Fatal(err.Error())
		}
	}

	if err := tx.Commit(); err != nil {
		log.Fatal(err.Error())
	}
}

func (s *StorageConn) Send(src, dst uuid.UUID, amount float64) error {
	tx, err := s.DB.BeginTx(context.Background(), nil)
	if err != nil {
		return err
	}

	var srcBalance int
	stmt := `SELECT balance FROM purses WHERE id = $1`
	err = tx.QueryRow(stmt, src).Scan(&srcBalance)
	if err != nil {
		tx.Rollback()
		return ErrWrongSender
	}

	var dstBalance int
	stmt = `SELECT balance FROM purses WHERE id = $1`
	err = tx.QueryRow(stmt, dst).Scan(&dstBalance)
	if err != nil {
		tx.Rollback()
		return ErrWrongReceiver
	}

	if srcBalance-int(amount*100) < 0 {
		tx.Rollback()
		return ErrNotEnoughMoney
	}

	stmt = `UPDATE purses SET balance = $1 WHERE id = $2`
	_, err = tx.Exec(stmt, srcBalance-int(amount*100), src)
	if err != nil {
		tx.Rollback()
		return err
	}

	stmt = `UPDATE purses SET balance = balance + $1 WHERE id = $2`
	_, err = tx.Exec(stmt, dstBalance+int(amount*100), dst)
	if err != nil {
		tx.Rollback()
		return err
	}

	stmt = `INSERT INTO transactions (sender, receiver, amount, time) VALUES ($1, $2, $3, NOW())`
	_, err = tx.Exec(stmt, src, dst, amount*100)
	if err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Commit(); err != nil {
		log.Fatal(err.Error())
	}
	return nil
}

func (s *StorageConn) GetLast(n int) ([]Transaction, error) {
	res := make([]Transaction, 0, 10)

	stmt := `SELECT id, sender, receiver, amount, time FROM transactions ORDER BY time DESC LIMIT $1`
	rows, err := s.DB.Query(stmt, n)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	for rows.Next() {
		var tx Transaction
		if err = rows.Scan(&tx.Id, &tx.Sender, &tx.Receiver, &tx.Amount, &tx.Time); err != nil {
			return nil, err
		}
		tx.Amount = tx.Amount / 100
		res = append(res, tx)
	}

	return res, nil
}

func (s *StorageConn) GetBalance(id uuid.UUID) (Purse, error) {
	var p Purse
	p.Id = id

	stmt := `SELECT balance FROM purses WHERE id = $1`
	err := s.DB.QueryRow(stmt, id).Scan(&p.Balance)
	if err != nil {
		return Purse{}, ErrPurseNotFound
	}

	p.Balance = p.Balance / 100

	return p, nil
}
