package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/formeo/go-audio-converter/pkg/converter"
)

// runServer starts the HTTP API server
func runServer(host, port string) error {
	mux := http.NewServeMux()
	
	// API endpoints
	mux.HandleFunc("/", handleIndex)
	mux.HandleFunc("/health", handleHealth)
	mux.HandleFunc("/api/convert", handleConvert)
	mux.HandleFunc("/api/convert/mp3", handleConvertToMP3)
	mux.HandleFunc("/api/convert/wav", handleConvertToWAV)
	mux.HandleFunc("/api/info", handleInfo)
	mux.HandleFunc("/api/formats", handleFormats)
	
	// Apply middleware
	handler := corsMiddleware(loggingMiddleware(mux))
	
	addr := fmt.Sprintf("%s:%s", host, port)
	server := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	
	fmt.Printf("Starting server on http://%s\n", addr)
	fmt.Println("\nEndpoints:")
	fmt.Println("  GET  /              - Web interface")
	fmt.Println("  GET  /health        - Health check")
	fmt.Println("  POST /api/convert   - Convert audio (auto-detect)")
	fmt.Println("  POST /api/convert/mp3 - Convert to MP3")
	fmt.Println("  POST /api/convert/wav - Convert to WAV")
	fmt.Println("  POST /api/info      - Get file info")
	fmt.Println("  GET  /api/formats   - List supported formats")
	
	return server.ListenAndServe()
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(indexHTML))
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "ok",
		"version": version,
		"time":    time.Now().UTC().Format(time.RFC3339),
	})
}

func handleFormats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"input":  converter.SupportedInputFormats(),
		"output": converter.SupportedOutputFormats(),
	})
}

func handleConvert(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	// Get target format from query or default to mp3
	targetFmt := r.URL.Query().Get("format")
	if targetFmt == "" {
		targetFmt = "mp3"
	}
	
	outputFmt := converter.Format(targetFmt)
	if outputFmt != converter.FormatMP3 && outputFmt != converter.FormatWAV {
		http.Error(w, "Unsupported output format. Use 'mp3' or 'wav'", http.StatusBadRequest)
		return
	}
	
	processConversion(w, r, outputFmt)
}

func handleConvertToMP3(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	processConversion(w, r, converter.FormatMP3)
}

func handleConvertToWAV(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	processConversion(w, r, converter.FormatWAV)
}

func processConversion(w http.ResponseWriter, r *http.Request, outputFmt converter.Format) {
	// Parse multipart form (max 200MB)
	if err := r.ParseMultipartForm(200 << 20); err != nil {
		jsonError(w, "Failed to parse form: "+err.Error(), http.StatusBadRequest)
		return
	}
	
	// Get uploaded file
	file, header, err := r.FormFile("file")
	if err != nil {
		jsonError(w, "No file provided. Use 'file' form field.", http.StatusBadRequest)
		return
	}
	defer file.Close()
	
	// Detect input format
	ext := filepath.Ext(header.Filename)
	inputFmt := converter.DetectFormat(header.Filename)
	if inputFmt == converter.FormatUnknown {
		jsonError(w, "Unsupported input format: "+ext, http.StatusBadRequest)
		return
	}
	
	// Check if conversion is supported
	if !converter.CanConvert(inputFmt, outputFmt) {
		jsonError(w, fmt.Sprintf("Cannot convert %s to %s", inputFmt, outputFmt), http.StatusBadRequest)
		return
	}
	
	// Create temp file for input (some decoders need seeking)
	tmpIn, err := os.CreateTemp("", "audioconv-in-*"+ext)
	if err != nil {
		jsonError(w, "Server error: cannot create temp file", http.StatusInternalServerError)
		return
	}
	defer os.Remove(tmpIn.Name())
	defer tmpIn.Close()
	
	if _, err := io.Copy(tmpIn, file); err != nil {
		jsonError(w, "Failed to save uploaded file", http.StatusInternalServerError)
		return
	}
	tmpIn.Seek(0, 0)
	
	// Create temp file for output
	tmpOut, err := os.CreateTemp("", "audioconv-out-*."+string(outputFmt))
	if err != nil {
		jsonError(w, "Server error: cannot create output file", http.StatusInternalServerError)
		return
	}
	defer os.Remove(tmpOut.Name())
	defer tmpOut.Close()
	
	// Parse options from query params
	opts := parseOptionsFromRequest(r)
	conv := converter.NewWithOptions(opts)
	
	// Convert
	if err := conv.Convert(tmpIn, tmpOut, inputFmt, outputFmt); err != nil {
		jsonError(w, "Conversion failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	
	// Send result
	tmpOut.Seek(0, 0)
	stat, _ := tmpOut.Stat()
	
	outFilename := header.Filename[:len(header.Filename)-len(ext)] + "." + string(outputFmt)
	w.Header().Set("Content-Type", getMimeType(outputFmt))
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, outFilename))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", stat.Size()))
	w.Header().Set("X-Original-Filename", header.Filename)
	w.Header().Set("X-Output-Filename", outFilename)
	
	io.Copy(w, tmpOut)
}

func handleInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	// Parse multipart form
	if err := r.ParseMultipartForm(200 << 20); err != nil {
		jsonError(w, "Failed to parse form", http.StatusBadRequest)
		return
	}
	
	file, header, err := r.FormFile("file")
	if err != nil {
		jsonError(w, "No file provided", http.StatusBadRequest)
		return
	}
	defer file.Close()
	
	// Detect format
	format := converter.DetectFormat(header.Filename)
	if format == converter.FormatUnknown {
		jsonError(w, "Unknown audio format", http.StatusBadRequest)
		return
	}
	
	// Read file for analysis
	tmpIn, err := os.CreateTemp("", "audioconv-info-*"+filepath.Ext(header.Filename))
	if err != nil {
		jsonError(w, "Server error", http.StatusInternalServerError)
		return
	}
	defer os.Remove(tmpIn.Name())
	defer tmpIn.Close()
	
	io.Copy(tmpIn, file)
	tmpIn.Seek(0, 0)
	
	info, err := converter.GetInfo(tmpIn, format)
	if err != nil {
		jsonError(w, "Failed to analyze file: "+err.Error(), http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"filename":    header.Filename,
		"size":        header.Size,
		"format":      info.Format,
		"duration":    info.Duration,
		"sample_rate": info.SampleRate,
		"channels":    info.Channels,
		"bit_depth":   info.BitDepth,
	})
}

// Helpers

func parseOptionsFromRequest(r *http.Request) converter.Options {
	opts := converter.DefaultOptions()
	
	if v := r.URL.Query().Get("bitrate"); v != "" {
		fmt.Sscanf(v, "%d", &opts.Bitrate)
	}
	if v := r.URL.Query().Get("sample_rate"); v != "" {
		fmt.Sscanf(v, "%d", &opts.SampleRate)
	}
	if v := r.URL.Query().Get("channels"); v != "" {
		fmt.Sscanf(v, "%d", &opts.Channels)
	}
	if r.URL.Query().Get("normalize") == "true" {
		opts.Normalize = true
	}
	if r.URL.Query().Get("trim_silence") == "true" {
		opts.TrimSilence = true
	}
	
	return opts
}

func getMimeType(fmt converter.Format) string {
	switch fmt {
	case converter.FormatMP3:
		return "audio/mpeg"
	case converter.FormatWAV:
		return "audio/wav"
	case converter.FormatFLAC:
		return "audio/flac"
	case converter.FormatOGG:
		return "audio/ogg"
	default:
		return "application/octet-stream"
	}
}

func jsonError(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

// Middleware

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		if !quiet {
			fmt.Printf("%s %s %s %v\n",
				r.Method,
				r.URL.Path,
				r.RemoteAddr,
				time.Since(start).Round(time.Millisecond),
			)
		}
	})
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept")
		
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		
		next.ServeHTTP(w, r)
	})
}

// HTML template for web interface
const indexHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Audio Converter</title>
    <style>
        * { box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            max-width: 800px;
            margin: 0 auto;
            padding: 40px 20px;
            background: #f5f5f5;
        }
        h1 { color: #333; margin-bottom: 10px; }
        .subtitle { color: #666; margin-bottom: 30px; }
        .card {
            background: white;
            border-radius: 8px;
            padding: 30px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
            margin-bottom: 20px;
        }
        .drop-zone {
            border: 2px dashed #ccc;
            border-radius: 8px;
            padding: 40px;
            text-align: center;
            cursor: pointer;
            transition: all 0.3s;
        }
        .drop-zone:hover, .drop-zone.dragover {
            border-color: #007bff;
            background: #f8f9ff;
        }
        .drop-zone input { display: none; }
        .options { margin: 20px 0; }
        .options label {
            display: block;
            margin: 10px 0;
        }
        .options select, .options input[type="number"] {
            padding: 8px;
            border: 1px solid #ddd;
            border-radius: 4px;
            margin-left: 10px;
        }
        .btn {
            background: #007bff;
            color: white;
            border: none;
            padding: 12px 24px;
            border-radius: 4px;
            cursor: pointer;
            font-size: 16px;
        }
        .btn:hover { background: #0056b3; }
        .btn:disabled { background: #ccc; cursor: not-allowed; }
        .result {
            margin-top: 20px;
            padding: 15px;
            border-radius: 4px;
        }
        .result.success { background: #d4edda; color: #155724; }
        .result.error { background: #f8d7da; color: #721c24; }
        .progress {
            height: 4px;
            background: #e9ecef;
            border-radius: 2px;
            margin-top: 10px;
            overflow: hidden;
        }
        .progress-bar {
            height: 100%;
            background: #007bff;
            width: 0;
            transition: width 0.3s;
        }
        .file-info { color: #666; margin-top: 10px; }
    </style>
</head>
<body>
    <h1>🎵 Audio Converter</h1>
    <p class="subtitle">Convert audio files without ffmpeg • Pure Go</p>
    
    <div class="card">
        <div class="drop-zone" id="dropZone">
            <p>📁 Drop audio file here or click to select</p>
            <p style="color: #999; font-size: 14px;">Supports: WAV, FLAC, OGG, MP3</p>
            <input type="file" id="fileInput" accept=".wav,.flac,.ogg,.mp3,.wave,.oga">
        </div>
        <div class="file-info" id="fileInfo"></div>
        
        <div class="options">
            <label>
                Output format:
                <select id="format">
                    <option value="mp3">MP3</option>
                    <option value="wav">WAV</option>
                </select>
            </label>
            <label>
                <input type="checkbox" id="normalize"> Normalize audio levels
            </label>
            <label>
                <input type="checkbox" id="trimSilence"> Trim silence
            </label>
        </div>
        
        <button class="btn" id="convertBtn" disabled>Convert</button>
        
        <div class="progress" id="progress" style="display:none">
            <div class="progress-bar" id="progressBar"></div>
        </div>
        
        <div class="result" id="result" style="display:none"></div>
    </div>
    
    <div class="card">
        <h3>API Usage</h3>
        <pre style="background:#f5f5f5;padding:15px;border-radius:4px;overflow-x:auto">
# Convert to MP3
curl -X POST -F "file=@input.wav" http://localhost:8080/api/convert/mp3 -o output.mp3

# Convert with options
curl -X POST -F "file=@input.flac" \
  "http://localhost:8080/api/convert?format=mp3&normalize=true" \
  -o output.mp3

# Get file info
curl -X POST -F "file=@audio.wav" http://localhost:8080/api/info</pre>
    </div>
    
    <script>
        const dropZone = document.getElementById('dropZone');
        const fileInput = document.getElementById('fileInput');
        const fileInfo = document.getElementById('fileInfo');
        const convertBtn = document.getElementById('convertBtn');
        const progress = document.getElementById('progress');
        const progressBar = document.getElementById('progressBar');
        const result = document.getElementById('result');
        
        let selectedFile = null;
        
        dropZone.onclick = () => fileInput.click();
        
        dropZone.ondragover = (e) => {
            e.preventDefault();
            dropZone.classList.add('dragover');
        };
        
        dropZone.ondragleave = () => dropZone.classList.remove('dragover');
        
        dropZone.ondrop = (e) => {
            e.preventDefault();
            dropZone.classList.remove('dragover');
            if (e.dataTransfer.files.length) {
                handleFile(e.dataTransfer.files[0]);
            }
        };
        
        fileInput.onchange = () => {
            if (fileInput.files.length) {
                handleFile(fileInput.files[0]);
            }
        };
        
        function handleFile(file) {
            selectedFile = file;
            fileInfo.textContent = file.name + ' (' + formatSize(file.size) + ')';
            convertBtn.disabled = false;
            result.style.display = 'none';
        }
        
        convertBtn.onclick = async () => {
            if (!selectedFile) return;
            
            const format = document.getElementById('format').value;
            const normalize = document.getElementById('normalize').checked;
            const trimSilence = document.getElementById('trimSilence').checked;
            
            const formData = new FormData();
            formData.append('file', selectedFile);
            
            let url = '/api/convert/' + format + '?';
            if (normalize) url += 'normalize=true&';
            if (trimSilence) url += 'trim_silence=true&';
            
            convertBtn.disabled = true;
            progress.style.display = 'block';
            progressBar.style.width = '30%';
            result.style.display = 'none';
            
            try {
                const response = await fetch(url, {
                    method: 'POST',
                    body: formData
                });
                
                progressBar.style.width = '80%';
                
                if (!response.ok) {
                    const error = await response.json();
                    throw new Error(error.error || 'Conversion failed');
                }
                
                const blob = await response.blob();
                const filename = response.headers.get('X-Output-Filename') || 'output.' + format;
                
                progressBar.style.width = '100%';
                
                // Download
                const a = document.createElement('a');
                a.href = URL.createObjectURL(blob);
                a.download = filename;
                a.click();
                
                result.className = 'result success';
                result.textContent = '✓ Converted: ' + filename + ' (' + formatSize(blob.size) + ')';
                result.style.display = 'block';
            } catch (err) {
                result.className = 'result error';
                result.textContent = '✗ Error: ' + err.message;
                result.style.display = 'block';
            } finally {
                convertBtn.disabled = false;
                setTimeout(() => {
                    progress.style.display = 'none';
                    progressBar.style.width = '0';
                }, 500);
            }
        };
        
        function formatSize(bytes) {
            if (bytes < 1024) return bytes + ' B';
            if (bytes < 1024*1024) return (bytes/1024).toFixed(1) + ' KB';
            return (bytes/1024/1024).toFixed(1) + ' MB';
        }
    </script>
</body>
</html>`
