package chapter_downloader

import (
	"encoding/xml"
	"io"
	"sort"
)

// ComicInfo is the standard CBZ metadata document (ComicInfo.xml) embedded in
// each downloaded chapter archive. Only the fields Seanime produces or consumes
// are modeled; unknown elements are ignored when parsing foreign archives.
type (
	ComicInfo struct {
		XMLName   xml.Name        `xml:"ComicInfo"`
		XsiNs     string          `xml:"xmlns:xsi,attr,omitempty"`
		XsdNs     string          `xml:"xmlns:xsd,attr,omitempty"`
		Title     string          `xml:"Title,omitempty"`
		Series    string          `xml:"Series,omitempty"`
		Number    string          `xml:"Number,omitempty"`
		PageCount int             `xml:"PageCount,omitempty"`
		Pages     *ComicInfoPages `xml:"Pages,omitempty"`
	}

	ComicInfoPages struct {
		Pages []ComicInfoPage `xml:"Page"`
	}

	ComicInfoPage struct {
		// Image is the 0-based position of the page within the archive.
		Image       int   `xml:"Image,attr"`
		ImageSize   int64 `xml:"ImageSize,attr,omitempty"`
		ImageWidth  int   `xml:"ImageWidth,attr,omitempty"`
		ImageHeight int   `xml:"ImageHeight,attr,omitempty"`
	}
)

const (
	comicInfoFilename = "ComicInfo.xml"
	comicInfoXsiNs    = "http://www.w3.org/2001/XMLSchema-instance"
	comicInfoXsdNs    = "http://www.w3.org/2001/XMLSchema"
)

// buildComicInfo assembles the ComicInfo document for a chapter from the page
// registry. Titles may be empty (e.g. during offline migration).
func buildComicInfo(id DownloadID, mediaTitle string, chapterTitle string, registry Registry) *ComicInfo {
	pages := make([]ComicInfoPage, 0, len(registry))
	for _, pageInfo := range registry {
		pages = append(pages, ComicInfoPage{
			Image:       pageInfo.Index,
			ImageSize:   pageInfo.Size,
			ImageWidth:  pageInfo.Width,
			ImageHeight: pageInfo.Height,
		})
	}
	sort.Slice(pages, func(i, j int) bool { return pages[i].Image < pages[j].Image })
	// Re-number positions so the Image attribute always matches the archive
	// order even if the registry indexes are not contiguous.
	for i := range pages {
		pages[i].Image = i
	}

	return &ComicInfo{
		XsiNs:     comicInfoXsiNs,
		XsdNs:     comicInfoXsdNs,
		Title:     chapterTitle,
		Series:    mediaTitle,
		Number:    id.ChapterNumber,
		PageCount: len(pages),
		Pages:     &ComicInfoPages{Pages: pages},
	}
}

func (ci *ComicInfo) marshal() ([]byte, error) {
	data, err := xml.MarshalIndent(ci, "", "  ")
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), data...), nil
}

func parseComicInfo(r io.Reader) (*ComicInfo, error) {
	var ci ComicInfo
	if err := xml.NewDecoder(r).Decode(&ci); err != nil {
		return nil, err
	}
	return &ci, nil
}
