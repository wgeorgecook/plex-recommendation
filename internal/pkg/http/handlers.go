package httpinternal

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/wgeorgecook/plex-recommendation/internal/pkg/langchain"
	"github.com/wgeorgecook/plex-recommendation/internal/pkg/plex"
)

type response struct {
	Response string `json:"response"`
}

func formatHttpError(err error) []byte {
	return []byte(fmt.Sprintf(`{"error": "%s"}`, err.Error()))
}

func getRecommendation(w http.ResponseWriter, r *http.Request) {
	section := r.PathValue("movieSection")
	var limit int
	limitQuery, ok := r.URL.Query()["limit"]
	if ok {
		limit, _ = strconv.Atoi(limitQuery[0])
	}

	recentlyViewed, err := plex.GetRecentlyPlayed(plexClient, section, limit)
	if err != nil {
		w.Write(formatHttpError(err))
		return
	}

	rvStr := buildStringFromSlice(recentlyViewed)

	fullCollection, err := plex.GetAllVideos(plexClient, section)
	if err != nil {
		w.Write(formatHttpError(err))
		return
	}

	fcStr := buildStringFromSlice(fullCollection)

	runSimple := os.Getenv("RUN_SIMPLE")
	full := runSimple == ""
	var recommendation string
	if full {

		recommendation, err = langchain.GenerateRecommendation(context.Background(), rvStr, fcStr, &ollamaLlm)

	} else {
		recommendation, err = langchain.GenerateSimpleRecommendation(context.Background(), &ollamaLlm)
	}
	if err != nil {
		w.Write(formatHttpError(err))
		return
	}

	resp := response{
		Response: recommendation,
	}
	respBytes, err := json.Marshal(&resp)
	if err != nil {
		w.Write(formatHttpError(err))
		return
	}

	_, err = w.Write(respBytes)
	if err != nil {
		log.Println("could not write back to client: ", err.Error())
	}
}