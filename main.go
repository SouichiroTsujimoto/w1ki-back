package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/url"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2/middleware/cors"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	_ "github.com/mattn/go-sqlite3"
)

type Page struct {
	Title    string `json:"title"`
	Markdown string `json:"markdown"`
}

func (p *Page) String() string {
	b, _ := json.Marshal(p)
	return string(b)
}

func toPage(s string, p *Page) {
	if err := json.Unmarshal([]byte(s), &p); err != nil {
		panic(err)
	}
}

type ConnInfo struct {
	conn *websocket.Conn
	page string
}

type WsConns struct {
	conns map[string]ConnInfo
}

func NewWsConns() *WsConns {
	return &WsConns{
		conns: make(map[string]ConnInfo),
	}
}

func (c *WsConns) Add(conn *websocket.Conn, title string) string {
	id := uuid.NewString()
	c.conns[id] = ConnInfo{
		conn,
		title,
	}

	return id
}

func (c *WsConns) Remove(id string) {
	if connInfo, ok := c.conns[id]; ok {
		delete(c.conns, id)
		connInfo.conn.Close()
	}
}

func (c *WsConns) NotifyNewConnection(myid string) {
	connectionCount := 1

	for id, connInfo := range c.conns {
		if id == myid {
			continue
		}
		if connInfo.page != c.conns[myid].page {
			continue
		}
		connectionCount++
		if err := connInfo.conn.WriteMessage(websocket.TextMessage, []byte("NewConnection:")); err != nil {
			c.Remove(id)
		}
	}

	c.SendConnectionsToAll([]byte(strconv.Itoa(connectionCount)), myid)
}

func (c *WsConns) SendConnectionsToAll(message []byte, myid string) {
	for id, conninfo := range c.conns {
		if conninfo.page != c.conns[myid].page {
			continue
		}
		if err := conninfo.conn.WriteMessage(websocket.TextMessage, []byte("Connections:"+string(message))); err != nil {
			c.Remove(id)
		}
	}
}

func (c *WsConns) SendMessageToOthers(message []byte, myid string) {
	for id, conninfo := range c.conns {
		if id == myid {
			continue
		}
		if conninfo.page != c.conns[myid].page {
			continue
		}
		if err := conninfo.conn.WriteMessage(websocket.TextMessage, []byte("Message:"+string(message))); err != nil {
			c.Remove(id)
		}
	}
}

// SQLiteStorage は直接SQLiteを操作するカスタムストレージ
type SQLiteStorage struct {
	db *sql.DB
}

// NewSQLiteStorage は新しいSQLiteStorageを作成します
func NewSQLiteStorage(db *sql.DB) *SQLiteStorage {
	return &SQLiteStorage{db: db}
}

// Get はキーに対応する値を取得します
func (s *SQLiteStorage) Get(key string) ([]byte, error) {
	//encodedKey := url.QueryEscape(key)

	var content []byte
	err := s.db.QueryRow("SELECT content FROM pages WHERE title = ?", key).Scan(&content)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return content, err
}

// Set はキーに対応する値を設定します
func (s *SQLiteStorage) Set(key string, val []byte, _ time.Duration) error {
	//encodedKey := url.QueryEscape(key)
	// UPSERTを使用してデータを挿入または更新
	_, err := s.db.Exec(
		"INSERT INTO pages (title, content, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP) ON CONFLICT(title) DO UPDATE SET content = ?, updated_at = CURRENT_TIMESTAMP",
		key, val, val,
	)
	return err
}

// Delete はキーに対応する値を削除します
func (s *SQLiteStorage) Delete(key string) error {
	//encodedKey := url.QueryEscape(key)

	_, err := s.db.Exec("DELETE FROM pages WHERE title = ?", key)
	return err
}

// Reset はすべてのデータを削除します
func (s *SQLiteStorage) Reset() error {
	_, err := s.db.Exec("DELETE FROM pages")
	return err
}

// Close はデータベース接続をクローズします
func (s *SQLiteStorage) Close() error {
	return s.db.Close()
}

// Keys はすべてのキーのリストを返します
func (s *SQLiteStorage) Keys() ([]string, error) {
	rows, err := s.db.Query("SELECT title FROM pages")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	return keys, rows.Err()
}

func main() {
	// SQLiteデータベース接続を初期化
	db, err := sql.Open("sqlite3", "./wiki.db")
	if err != nil {
		log.Fatalf("データベース接続エラー: %v", err)
	}
	defer db.Close()

	// テーブルが存在しない場合は作成
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS pages (
		title TEXT PRIMARY KEY,
		content BLOB,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		log.Fatalf("テーブル作成エラー: %v", err)
	}

	// カスタムストレージを初期化
	store := NewSQLiteStorage(db)

	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			return c.Status(code).JSON(fiber.Map{
				"error": err.Error(),
			})
		},
	})

	app.Use(cors.New(cors.Config{
		AllowOrigins:     "https://w1ki-demo.vercel.app/",
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization",
		AllowMethods:     "GET, POST, PUT, DELETE, OPTIONS",
		AllowCredentials: true,
	}))

	wsconns := NewWsConns()

	app.Get("/", func(c *fiber.Ctx) error {
		keys, err := store.Keys()
		if err != nil {
			return err
		}
		decodedKeys := make([]string, len(keys))
		for i, key := range keys {
			decodedKeys[i], _ = url.QueryUnescape(key)
		}
		return c.JSON(decodedKeys)
	})

	app.Get("/ws/:title", websocket.New(func(c *websocket.Conn) {
		title := c.Params("title")

		requestId := wsconns.Add(c, title)
		defer wsconns.Remove(requestId)

		log.Printf("新しいWebSocket接続: %s", requestId)

		wsconns.NotifyNewConnection(requestId)

		for {
			messageType, message, err := c.ReadMessage()
			if err != nil {
				log.Printf("メッセージ読み取りエラー: %v", err)
				break
			}

			if messageType == websocket.CloseMessage || messageType == websocket.CloseGoingAway {
				log.Printf("WebSocket接続が閉じられました: %s", requestId)
				break
			}

			if messageType == websocket.TextMessage {
				log.Printf("メッセージ %s", message)
				wsconns.SendMessageToOthers(message, requestId)
			}
		}
	}))

	app.Get("/page/:title", func(c *fiber.Ctx) error {
		title := c.Params("title")
		log.Println(title)
		page, err := store.Get(title)
		if err != nil {
			return err
		}
		return c.SendString(string(page))
	})

	app.Post("/page/:title", func(c *fiber.Ctx) error {
		title := c.Params("title")
		page := Page{}
		toPage(string(c.Body()), &page)
		log.Println(string(c.Body()))
		log.Println(page.Title)
		log.Println(url.QueryEscape(page.Title))
		log.Println(title)
		log.Println(page.Markdown)
		log.Println(page.String())
		//if page.Title != title {
		//	return fiber.NewError(fiber.StatusBadRequest, "title is not match")
		//}
		// 未登録ならdbに登録
		if err := store.Set(title, []byte(page.Markdown), 0); err != nil {
			return err
		}

		return c.SendString(page.String())
	})

	app.Delete("/page/:title", func(c *fiber.Ctx) error {
		title := c.Params("title")
		log.Println("Delete: " + title + "(" + c.Params("title") + ")")
		if err := store.Delete(title); err != nil {
			return err
		}

		return c.SendStatus(fiber.StatusOK)
	})

	log.Println("バックエンド: http://localhost:8080")
	if err := app.Listen(":8080"); err != nil {
		log.Fatalf("サーバー開始エラー: %s", err.Error())
	}
}
