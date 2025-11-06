package internal

import (
	"image/color"
	"slices"

	"github.com/disintegration/imaging"
)

type AttachmentDiffs struct {
	ToAdd    []string
	ToDelete []string
}

func DiffAttachments(postIds, dbIds []string) AttachmentDiffs {
	diffs := AttachmentDiffs{}

	for _, id := range postIds {
		if !slices.Contains(dbIds, id) {
			diffs.ToAdd = append(diffs.ToAdd, id)
		}
	}

	for _, id := range dbIds {
		if !slices.Contains(postIds, id) {
			diffs.ToDelete = append(diffs.ToDelete, id)
		}
	}

	return diffs
}

func CreateThumbnail(originalFilePath string, width, height int, thumbFilePath string) error {
	img, err := imaging.Open(originalFilePath, imaging.AutoOrientation(true))

	if err != nil {
		return err
	}

	thumbnail := imaging.Thumbnail(img, width, height, imaging.CatmullRom)

	// create a new blank image
	dst := imaging.New(width, height, color.NRGBA{})

	// paste thumbnails into the new image side by side
	dst = imaging.PasteCenter(dst, thumbnail)

	// save the combined image to file
	err = imaging.Save(dst, thumbFilePath)

	return err
}
