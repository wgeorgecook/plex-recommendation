package plex

import (
	"context"
	"encoding/xml"
	"io"
	"log"
	"net/http"

	"github.com/wgeorgecook/plex-recommendation/internal/pkg/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

const allMovies = true

type MediaContainer struct {
	XMLName             xml.Name `xml:"MediaContainer"`
	Size                int      `xml:"size,attr"`
	AllowSync           int      `xml:"allowSync,attr"`
	Art                 string   `xml:"art,attr"`
	Identifier          string   `xml:"identifier,attr"`
	LibrarySectionID    int      `xml:"librarySectionID,attr"`
	LibrarySectionTitle string   `xml:"librarySectionTitle,attr"`
	// ... (other MediaContainer attributes)
	Videos []Video `xml:"Video"`
}

type Video struct {
	XMLName       xml.Name `xml:"Video"`
	RatingKey     int      `xml:"ratingKey,attr"`
	Key           string   `xml:"key,attr"`
	Guid          string   `xml:"guid,attr"`
	Slug          string   `xml:"slug,attr"`
	Studio        string   `xml:"studio,attr"`
	Type          string   `xml:"type,attr"`
	Title         string   `xml:"title,attr"`
	ContentRating string   `xml:"contentRating,attr"`
	Summary       string   `xml:"summary,attr"`
}

type VideoShort struct {
	Title         string `json:"title"`
	Summary       string `json:"summary"`
	ContentRating string `json:"content_rating"`
	PlexID        string `json:"plex_id"`
}

func (v VideoShort) String() string {
	return "Title: " + v.Title +
		"\nSummary: " + v.Summary +
		"\nContent Rating: " + v.ContentRating +
		"\nPlex ID: " + v.PlexID
}

func fullToShort(vids []Video, limit int) []VideoShort {
	shorts := make([]VideoShort, 0, limit)
	for i, vid := range vids {
		if i >= limit {
			break
		}
		shorts = append(shorts, VideoShort{
			Title:         vid.Title,
			Summary:       vid.Summary,
			ContentRating: vid.ContentRating,
			PlexID:        vid.Guid,
		})
	}

	return shorts
}

func GetRecentlyPlayed(ctx context.Context, c Client, sectionId string, limit int) ([]VideoShort, error) {
	ctx, span := telemetry.StartSpan(ctx, telemetry.WithSpanName("GetRecentlyPlayed"))
	defer span.End()
	log.Println("connecting to Plex...")
	connectionUri := c.Connect(WithSectionID(sectionId))
	log.Println("connected")
	span.AddEvent("connected to Plex")

	log.Println("getting recently watched...")
	resp, err := c.MakeNetworkRequest(ctx, connectionUri, http.MethodGet)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}
	log.Println("received")
	span.AddEvent("Received recently watched")

	var container MediaContainer
	if err := xml.Unmarshal(bodyBytes, &container); err != nil {
		span.RecordError(err)
		return nil, err
	}
	log.Printf("total count: %v\n", len(container.Videos))
	span.SetAttributes(attribute.Int("total count", len(container.Videos)))
	shorts := fullToShort(container.Videos, limit)
	log.Printf("returning %v recently watched\n", limit)
	span.SetStatus(codes.Ok, "recently watched complete")
	return shorts, nil
}

func GetAllVideos(ctx context.Context, c Client, sectionId string) ([]VideoShort, error) {
	ctx, span := telemetry.StartSpan(ctx, telemetry.WithSpanName("GetAllVideos"))
	defer span.End()

	span.SetAttributes(attribute.String("package", "plex"))
	log.Println("connecting to Plex...")
	connectionUri := c.Connect(WithSectionID(sectionId), WithAllMovies(allMovies))
	log.Println("connected")
	span.AddEvent("connected to plex")

	log.Printf("getting all videos from %s...\n", connectionUri)
	resp, err := c.MakeNetworkRequest(ctx, connectionUri, http.MethodGet)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}
	log.Println("received")

	var container MediaContainer
	if err := xml.Unmarshal(bodyBytes, &container); err != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.String("received bytes: ", string(bodyBytes)))
		log.Println("expected MediaContainer but received ", string(bodyBytes))
		return nil, err
	}
	log.Printf("total count: %v\n", len(container.Videos))
	shorts := fullToShort(container.Videos, len(container.Videos))
	log.Println("returning all movies")
	span.SetStatus(codes.Ok, "all movies complete")
	return shorts, nil
}
