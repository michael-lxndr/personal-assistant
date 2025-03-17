package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func main() {
	http.HandleFunc("/convert", handleConvert)

	port := getEnvOr("PORT", "8080")
	log.Printf("Servidor iniciado en puerto %s...\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func getEnvOr(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func handleConvert(w http.ResponseWriter, r *http.Request) {
	// Solo permitir método POST
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}

	// Leer directamente del cuerpo de la solicitud
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error al leer el cuerpo de la solicitud: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if len(body) == 0 {
		http.Error(w, "Cuerpo de la solicitud vacío", http.StatusBadRequest)
		return
	}

	log.Printf("Recibido archivo de %d bytes\n", len(body))

	// Crear directorio temporal si no existe
	tempDir := "./temp"
	if _, err := os.Stat(tempDir); os.IsNotExist(err) {
		os.Mkdir(tempDir, 0755)
	}

	// Generar nombres de archivo únicos basados en timestamp
	timestamp := time.Now().Unix()
	tempInputPath := filepath.Join(tempDir, fmt.Sprintf("input_%d.oga", timestamp))
	tempOutputPath := filepath.Join(tempDir, fmt.Sprintf("output_%d.mp3", timestamp))

	// Guardar el archivo recibido
	err = os.WriteFile(tempInputPath, body, 0644)
	if err != nil {
		http.Error(w, "Error al guardar archivo temporal: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Convertir de OGA a MP3 usando ffmpeg
	cmd := exec.Command("ffmpeg", "-i", tempInputPath, "-acodec", "libmp3lame", "-q:a", "2", tempOutputPath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		http.Error(w, fmt.Sprintf("Error en la conversión con ffmpeg: %v\n%s", err, stderr.String()), http.StatusInternalServerError)
		os.Remove(tempInputPath)
		return
	}

	// Abrir el archivo MP3 generado
	convertedFile, err := os.Open(tempOutputPath)
	if err != nil {
		http.Error(w, "Error al abrir el archivo convertido: "+err.Error(), http.StatusInternalServerError)
		cleanup(tempInputPath, tempOutputPath)
		return
	}
	defer convertedFile.Close()

	// Obtener información del archivo
	fileInfo, err := convertedFile.Stat()
	if err != nil {
		http.Error(w, "Error al obtener información del archivo: "+err.Error(), http.StatusInternalServerError)
		cleanup(tempInputPath, tempOutputPath)
		return
	}

	// Configurar encabezados para descarga
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=converted_%d.mp3", timestamp))
	w.Header().Set("Content-Type", "audio/mpeg")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))

	// Enviar el archivo al cliente
	_, err = io.Copy(w, convertedFile)
	if err != nil {
		log.Printf("Error al enviar archivo al cliente: %v", err)
	}

	// Limpiar archivos temporales
	cleanup(tempInputPath, tempOutputPath)
	log.Printf("Conversión completada. Archivo temporal: %s\n", tempOutputPath)
}

func cleanup(files ...string) {
	for _, file := range files {
		os.Remove(file)
	}
}
