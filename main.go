package main

import (
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/hlog"
	"gopkg.in/natefinch/lumberjack.v2"
)

var httpPort = flag.Int("port", 8080, "listen port")

type bodyRequest struct {
	Text string `json:"text"`
}

type bodyResp struct {
	Translated string `json:"translated"`
}

func main() {
	flag.Parse()

	log := zerolog.New(io.MultiWriter(zerolog.NewConsoleWriter(), &lumberjack.Logger{
		Filename: "./main.log",
		MaxSize:  100,
		MaxAge:   7,
	})).With().Timestamp().Logger()

	api := TranslatorAPI{
		LogOut: &lumberjack.Logger{
			Filename: "./translator.log",
			MaxSize:  100,
			MaxAge:   7,
		},
	}
	if err := api.Init(); err != nil {
		log.Fatal().Err(err).Msg("Cannot initialize translator api")
	}
	defer api.Stop()

	mux := http.NewServeMux()
	mux.Handle("/", HTTPLogger(log, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(http.StatusText(http.StatusNotFound)))
	})))

	mux.Handle("/translate", HTTPLogger(log, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log := log
		if id, ok := hlog.IDFromRequest(r); ok {
			log = log.With().Stringer("req_id", id).Logger()
		}

		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			w.Write([]byte(http.StatusText(http.StatusMethodNotAllowed)))
			return
		}

		var req bodyRequest
		body, err := io.ReadAll(r.Body)
		if err != nil {
			log.Error().Err(err).Msg("Failed to reading request body")
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Got error when reading request body"))
			return
		}

		if err := json.Unmarshal(body, &req); err != nil {
			log.Warn().Err(err).Str("body", string(body)).Msg("Decode msg")
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Expected json body with field `text`"))
			return
		}

		translatedText, err := api.Translate(req.Text)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Sorry, I can't translate this :(\nThis is a server error"))
			log.Error().Err(err).Str("text_to_translate", translatedText).Msg("Cannot translate text")
		}

		log.Info().Str("origin_text", req.Text).Str("translated_text", translatedText).Msg("Request translated!")

		b, err := json.Marshal(bodyResp{Translated: translatedText})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Hmm, I can translate this\nBut there was some error"))
		}

		w.WriteHeader(http.StatusOK)
		w.Write(b)
	})))

	server := http.Server{
		Addr:    "localhost:" + strconv.Itoa(*httpPort),
		Handler: mux,
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error().Err(err).Msg("Failed to http listening")
			sig <- syscall.SIGTERM
		}
	}()

	log.Info().Int("port", *httpPort).Msg("App started")
	signal := <-sig
	log.Info().Stringer("signal", signal).Msg("Got signal!")

	if err := server.Close(); err != nil {
		log.Error().Err(err).Msg("Server closed bad :(")
	}

	wg.Wait()

	log.Info().Msg("Bye :)")
}
