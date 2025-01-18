package storage

import (
	"context"
	"database/sql"
	"errors"
	"github.com/google/uuid"
	"log"
)

var ErrNotEnoughMoney = errors.New("storage: недостаточно средств")
var ErrWrongSender = errors.New("storage: нет такого отправителя")
var ErrWrongReceiver = errors.New("storage: нет такого получателя")
var ErrPurseNotFound = errors.New("storage: нет такого кошелька")

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

// Init Функция инициализирует базу данных, создаёт таблицы purses и transactions.
// Также создаёт 10 произвольных кошельков с балансом 100 у.е.
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

// Send Функция производит перевод средств между двумя кошельками.
// Возвращает ошибку при любом неуспехе.
//
// Примечания:
// - Баланс хранится в целых числах для предотвращения проблем с округлением.
// - Функция предполагает, что передаваемые UUID и суммы валидны.
func (s *StorageConn) Send(src, dst uuid.UUID, amount float64) error {
	tx, err := s.DB.BeginTx(context.Background(), nil)
	if err != nil {
		return err
	}

	var srcBalance int
	stmt := `SELECT balance FROM purses WHERE id = $1`
	err = tx.QueryRow(stmt, src).Scan(&srcBalance)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			tx.Rollback()
			return ErrWrongSender
		}

		return err
	}

	var dstBalance int
	stmt = `SELECT balance FROM purses WHERE id = $1`
	err = tx.QueryRow(stmt, dst).Scan(&dstBalance)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			tx.Rollback()
			return ErrWrongReceiver
		}

		return err
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

// GetLast Функция возвращает n последних операций из таблицы transactions.
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

// GetBalance Функция возвращает id и баланс кошелька в структуре.
func (s *StorageConn) GetBalance(id uuid.UUID) (Purse, error) {
	var p Purse
	p.Id = id

	stmt := `SELECT balance FROM purses WHERE id = $1`
	err := s.DB.QueryRow(stmt, id).Scan(&p.Balance)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Purse{}, ErrPurseNotFound
		}

		return Purse{}, err
	}

	p.Balance = p.Balance / 100

	return p, nil
}
