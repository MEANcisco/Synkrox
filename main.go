package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/martinlindhe/notify" // Para Mac y Linux
	_ "github.com/mattn/go-sqlite3"
	_ "github.com/nakagami/firebirdsql"
)

const (
	dbHost       = "10.211.55.3"
	dbPort       = "3050"
	dbName       = "C:\\\\GesWinFB\\\\DATOS001.GDB"
	dbUser       = "SYSDBA"
	dbPassword   = "masterkey"
	sqliteDBPath = "synkros.db"
)

type Product struct {
	CODIGO_PRODUCTO  string   `json:"CODIGO_PRODUCTO"`
	NOMBRE_PRODUCTO  string   `json:"NOMBRE_PRODUCTO"`
	PREVTA1_PRODUCTO float64  `json:"PREVTA1_PRODUCTO"`
	AUTOR            string   `json:"AUTOR"`
	FOTO             []string `json:"FOTO"`
	FOTO_LENGTH      int      `json:"FOTO_LENGTH"`
}

var actionQueue []string // Cola de acciones pendientes

func initSQLite() *sql.DB {
	log.Println("Iniciando base de datos SQLite...")
	db, err := sql.Open("sqlite3", sqliteDBPath)
	if err != nil {
		log.Fatal("Error al abrir SQLite:", err)
	}

	createTableQuery := `
    CREATE TABLE IF NOT EXISTS sync_status (
        CODIGO_PRODUCTO TEXT PRIMARY KEY,
        last_synced TIMESTAMP,
        photo_size INTEGER,
        idCorrelativo INTEGER,
        NOMBRE_PRODUCTO TEXT,
        PREVTA1_PRODUCTO INTEGER
    );`
	_, err = db.Exec(createTableQuery)
	if err != nil {
		log.Fatal("Error al crear tabla de estado de sincronización:", err)
	}

	log.Println("Base de datos SQLite iniciada.")
	return db
}

func isProductSynced(db *sql.DB, codigo string) bool {
	var synced bool
	query := "SELECT EXISTS (SELECT 1 FROM sync_status WHERE CODIGO_PRODUCTO = ?)"
	err := db.QueryRow(query, codigo).Scan(&synced)
	if err != nil {
		log.Println("Error al verificar sincronización:", err)
		return false
	}
	return synced
}

func markProductAsSynced(db *sql.DB, codigo string, photoSize int, idCorrelativo int, NOMBRE_PRODUCTO string, PREVTA1_PRODUCTO float64) {
	query := "INSERT OR REPLACE INTO sync_status (CODIGO_PRODUCTO, last_synced, photo_size, idCorrelativo, NOMBRE_PRODUCTO, PREVTA1_PRODUCTO) VALUES (?, ?, ?, ?, ?, ?)"
	_, err := db.Exec(query, codigo, time.Now(), photoSize, idCorrelativo, NOMBRE_PRODUCTO, PREVTA1_PRODUCTO)
	if err != nil {
		log.Println("Error al marcar producto como sincronizado:", err)
	} else {
		log.Printf("Producto %s marcado como sincronizado con tamaño de foto %d bytes e ID %d.\n", codigo, photoSize, idCorrelativo)
	}
}

func fetchProducts(fbDB *sql.DB) ([]Product, error) {
	log.Println("Obteniendo productos desde Firebird...")
	rows, err := fbDB.Query("SELECT CODIGO_PRODUCTO, NOMBRE_PRODUCTO, PREVTA1_PRODUCTO, AUTOR FROM PRODUCTOS")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var products []Product
	for rows.Next() {
		var product Product
		err := rows.Scan(&product.CODIGO_PRODUCTO, &product.NOMBRE_PRODUCTO, &product.PREVTA1_PRODUCTO, &product.AUTOR)
		if err != nil {
			return nil, err
		}
		products = append(products, product)
	}
	log.Printf("Se obtuvieron %d productos.\n", len(products))
	return products, nil
}

func uploadAsset(filePath string, syncDB *sql.DB) (int, error) {
	log.Printf("Subiendo imagen para el asset desde %s...\n", filePath)

	file, err := os.Open(filePath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return 0, err
	}

	_, err = io.Copy(part, file)
	if err != nil {
		return 0, err
	}
	writer.Close()

	req, err := http.NewRequest("POST", "http://localhost:3000/products/uploadAsset", body)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		var result map[string]interface{}
		err := json.NewDecoder(resp.Body).Decode(&result)
		if err != nil {
			return 0, err
		}

		if idFloat, exists := result["id"].(float64); exists {
			id := int(idFloat)
			log.Printf("Asset subido exitosamente, ID: %d\n", id)
			return id, nil
		}
	}
	log.Println("Fallo al subir el asset.")
	return 0, fmt.Errorf("failed to upload asset")
}

func comparadorProductos(syncDB *sql.DB, product Product, photo []byte, nombre string, precio int) {
	var nomSync string
	var preSync int
	var phoSync []string
	var err = syncDB.QueryRow("SELECT * FROM sync_status WHERE CODIGO_PRODUCTO = ?", product.CODIGO_PRODUCTO).Scan(&nomSync, &preSync, &phoSync)
	productoActualizado := Product{
		CODIGO_PRODUCTO:  product.CODIGO_PRODUCTO,
		NOMBRE_PRODUCTO:  nombre,
		PREVTA1_PRODUCTO: float64(preSync),
		AUTOR:            "",
		FOTO:             phoSync,
		FOTO_LENGTH:      0,
	}
	if err != nil {
		return
	}

	if nomSync != nombre {
		log.Printf("Actualizando nombre de producto de: %s...\n", product.CODIGO_PRODUCTO)
		syncProduct(syncDB, productoActualizado, "update")

	}

	if preSync != precio {
		log.Printf("Actualizando el precio de:  %s...\n", product.CODIGO_PRODUCTO)
		syncProduct(syncDB, productoActualizado, "update")
	}
}
func fetchProductImage(fbDB *sql.DB, product Product, syncDB *sql.DB) (string, int, error) {
	log.Printf("Obteniendo imagen para el producto %s...\n", product.CODIGO_PRODUCTO)
	var photo []byte
	var nombre string
	var precio int
	err := fbDB.QueryRow("SELECT FOTO, NOMBRE_PRODUCTO, PREVTA1_PRODUCTO FROM PRODUCTOS WHERE CODIGO_PRODUCTO = ?", product.CODIGO_PRODUCTO).Scan(&photo, &nombre, &precio)

	if err != nil {
		return "", 0, err
	}

	if len(photo) > 0 {
		err = os.MkdirAll("temp", os.ModePerm)
		if err != nil {
			log.Printf("Error al crear el directorio 'temp': %v\n", err)
			return "", 0, err
		}

		filePath := fmt.Sprintf("temp/%s.jpeg", product.CODIGO_PRODUCTO)
		err = os.WriteFile(filePath, photo, 0644)
		if err != nil {
			return "", 0, err
		}

		fileInfo, err := os.Stat(filePath)
		if err != nil {
			log.Printf("Error al obtener el tamaño de la imagen: %v\n", err)
			return "", 0, err
		}
		fileSize := int(fileInfo.Size())

		log.Printf("Imagen guardada temporalmente en %s con tamaño %d bytes.\n", filePath, fileSize)
		return filePath, fileSize, nil
	}

	comparadorProductos(syncDB, product, photo, nombre, precio)
	log.Printf("No se encontró imagen para el producto %s.\n", product.CODIGO_PRODUCTO)
	return "", 0, nil
}

func syncProduct(db *sql.DB, product Product, syncType string) {
	jsonData, _ := json.Marshal(product)
	url := "http://localhost:3000/products"
	method := map[string]string{
		"new":    "POST",
		"delete": "DELETE",
		"update": "PUT",
	}[syncType]

	req, err := http.NewRequest(method, url, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Println("Error al crear solicitud:", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Println("Error al sincronizar producto:", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		var idCorrelativo int
		if len(product.FOTO) > 0 {
			idCorrelativo, _ = strconv.Atoi(product.FOTO[0])
		} else {
			idCorrelativo = 0
		}

		markProductAsSynced(db, product.CODIGO_PRODUCTO, product.FOTO_LENGTH, idCorrelativo, product.NOMBRE_PRODUCTO, product.PREVTA1_PRODUCTO)
		log.Printf("Producto %s sincronizado (%s).\n", product.CODIGO_PRODUCTO, syncType)
		actionQueue = actionQueue[:0]
	} else {
		log.Printf("Error al sincronizar producto (%s): %s\n", syncType, product.CODIGO_PRODUCTO)
	}
}

func checkActionQueue() string {
	if len(actionQueue) > 0 {
		return fmt.Sprintf("Acciones pendientes: %d", len(actionQueue))
	}
	return "Listo, en espera de novedades"
}

func showNotification() {
	message := checkActionQueue()
	notify.Notify("Estado de Sincronización", "Sincronización", message, "icono.png")
}

func syncProducts(fbDB *sql.DB, syncDB *sql.DB) {
	log.Println("Iniciando sincronización de productos...")
	products, err := fetchProducts(fbDB)
	if err != nil {
		log.Println("Error al obtener productos:", err)
		return
	}

	for _, product := range products {
		photoPath, photoSize, err := fetchProductImage(fbDB, product, syncDB)
		if err != nil {
			log.Printf("Error obteniendo imagen para %s: %v", product.CODIGO_PRODUCTO, err)
			continue
		}

		product.FOTO_LENGTH = photoSize

		var assetID int
		if photoPath != "" {
			if !isProductSynced(syncDB, product.CODIGO_PRODUCTO) {
				assetID, err = uploadAsset(photoPath, syncDB)
				if err != nil {
					log.Printf("Error al subir el asset para el producto %s: %v", product.CODIGO_PRODUCTO, err)
					continue
				}
			}
			product.FOTO = []string{fmt.Sprintf("%d", assetID)}
		} else {
			product.FOTO = []string{}
		}

		syncProduct(syncDB, product, "update")
	}
	log.Println("Sincronización de productos completada.")
}

func main() {
	log.Println("Conectando a la base de datos Firebird...")
	fbConnStr := fmt.Sprintf("%s:%s@%s/%s?role=RDB$ADMIN", dbUser, dbPassword, dbHost, dbName)
	fbDB, err := sql.Open("firebirdsql", fbConnStr)
	if err != nil {
		log.Fatal("Error al conectar a la base de datos Firebird:", err)
	}
	defer fbDB.Close()
	log.Println("Conectado a la base de datos Firebird.")

	syncDB := initSQLite()
	defer syncDB.Close()

	for {
		syncProducts(fbDB, syncDB)
		showNotification()
		log.Println("Esperando próximo ciclo de sincronización...")
		time.Sleep(1 * time.Hour)
	}
}
