package metadata

import (
	"context"
	"encoding/json"

	"github.com/GerardPolloRebozado/navifetch/src/util"
)

type CoverArtThumbnail struct {
	Num250  string `json:"250"`
	Num500  string `json:"500"`
	Num1200 string `json:"1200"`
	Small   string `json:"small"`
	Large   string `json:"large"`
}

type CoverArtResponse struct {
	Images []struct {
		Types      []string          `json:"types"`
		Front      bool              `json:"front"`
		Back       bool              `json:"back"`
		Edit       int               `json:"edit"`
		Image      string            `json:"image"`
		Comment    string            `json:"comment"`
		Approved   bool              `json:"approved"`
		ID         any               `json:"id"`
		Thumbnails CoverArtThumbnail `json:"thumbnails"`
	} `json:"images"`
	Release string `json:"release"`
}

func GetCoverArtArchive(ctx context.Context, ID string, size int64) (string, error) {
	body, _, _, err := util.HTTPGet(ctx, "https://coverartarchive.org/release/"+ID, nil)
	if err != nil {
		return "", err
	}
	var response CoverArtResponse
	err = json.Unmarshal(body, &response)
	if err != nil {
		return "", err
	}
	var thumbnails *CoverArtThumbnail = nil
	for _, image := range response.Images {
		if image.Front {
			thumbnails = &image.Thumbnails
		}
	}
	if thumbnails == nil {
		thumbnails = &response.Images[0].Thumbnails
	}
	switch {
	case size <= 250 && size >= 0:
		return thumbnails.Num250, nil
	case size <= 750 && size > 250:
		return thumbnails.Num500, nil
	default:
		return thumbnails.Num1200, nil
	}
}
