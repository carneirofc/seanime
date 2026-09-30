package chapter_downloader

import (
	"encoding/xml"
	"io"
	"sort"
)

// ComicInfo is the standard CBZ metadata document (ComicInfo.xml) embedded in
// each chapter archive Seanime writes. Only the fields Seanime produces or
// consumes are modeled; unknown elements are ignored when parsing foreign
// archives.
//
// Field order follows the ComicInfo v2.0 schema sequence, because readers that
// validate against the XSD reject an out-of-order document and Go marshals in
// declaration order. Everything but the identifying fields is optional, so a
// document only carries what the caller actually knows.
type (
	ComicInfo struct {
		XMLName xml.Name `xml:"ComicInfo"`
		XsiNs   string   `xml:"xmlns:xsi,attr,omitempty"`
		XsdNs   string   `xml:"xmlns:xsd,attr,omitempty"`
		Title   string   `xml:"Title,omitempty"`
		Series  string   `xml:"Series,omitempty"`
		Number  string   `xml:"Number,omitempty"`
		// Count is the total number of chapters in the series, when known.
		Count   int    `xml:"Count,omitempty"`
		Volume  string `xml:"Volume,omitempty"`
		Summary string `xml:"Summary,omitempty"`
		Notes   string `xml:"Notes,omitempty"`
		Year    int    `xml:"Year,omitempty"`
		Month   int    `xml:"Month,omitempty"`
		Day     int    `xml:"Day,omitempty"`
		// Writer, Penciller, Translator, Genre, Tags and Characters are
		// comma-separated lists, as the schema defines them.
		Writer    string `xml:"Writer,omitempty"`
		Penciller string `xml:"Penciller,omitempty"`
		// Translator is a ComicInfo v2.1 field; Seanime puts the scanlation group here.
		Translator string `xml:"Translator,omitempty"`
		Genre      string `xml:"Genre,omitempty"`
		Tags       string `xml:"Tags,omitempty"`
		// Web is a space-separated list of URLs.
		Web         string          `xml:"Web,omitempty"`
		PageCount   int             `xml:"PageCount,omitempty"`
		LanguageISO string          `xml:"LanguageISO,omitempty"`
		Manga       string          `xml:"Manga,omitempty"`
		Characters  string          `xml:"Characters,omitempty"`
		AgeRating   string          `xml:"AgeRating,omitempty"`
		Pages       *ComicInfoPages `xml:"Pages,omitempty"`
	}

	// ChapterMetadata is the scraped metadata of a queued chapter that ends up
	// in its ComicInfo.xml. It is persisted with the queue item as JSON, so it
	// survives a restart between queueing and downloading. Every field is
	// optional; the list fields are already comma-separated.
	ChapterMetadata struct {
		Summary     string `json:"summary,omitempty"`
		Notes       string `json:"notes,omitempty"`
		Year        int    `json:"year,omitempty"`
		Count       int    `json:"count,omitempty"`
		Writer      string `json:"writer,omitempty"`
		Penciller   string `json:"penciller,omitempty"`
		Translator  string `json:"translator,omitempty"`
		Genre       string `json:"genre,omitempty"`
		Tags        string `json:"tags,omitempty"`
		Web         string `json:"web,omitempty"`
		LanguageISO string `json:"languageIso,omitempty"`
		Characters  string `json:"characters,omitempty"`
		AgeRating   string `json:"ageRating,omitempty"`
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
	// ComicInfoFilename is the name the metadata document must have inside the
	// archive for readers to find it.
	ComicInfoFilename = "ComicInfo.xml"

	// MangaRightToLeft is the ComicInfo reading-direction value for manga.
	MangaRightToLeft = "YesAndRightToLeft"

	comicInfoXsiNs = "http://www.w3.org/2001/XMLSchema-instance"
	comicInfoXsdNs = "http://www.w3.org/2001/XMLSchema"
)

// NewComicInfo returns an empty document with the schema namespaces set.
func NewComicInfo() *ComicInfo {
	return &ComicInfo{XsiNs: comicInfoXsiNs, XsdNs: comicInfoXsdNs}
}

// SetPages fills in Pages and PageCount from the page list, renumbering the
// Image attributes so they always match the archive order.
func (ci *ComicInfo) SetPages(pages []ComicInfoPage) {
	sorted := make([]ComicInfoPage, len(pages))
	copy(sorted, pages)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Image < sorted[j].Image })
	for i := range sorted {
		sorted[i].Image = i
	}

	ci.PageCount = len(sorted)
	ci.Pages = &ComicInfoPages{Pages: sorted}
}

// buildComicInfo assembles the ComicInfo document for a downloaded chapter from
// the page registry. Titles and metadata may be empty (e.g. during offline
// migration, or for a queue item persisted before metadata was recorded).
func buildComicInfo(id DownloadID, mediaTitle string, chapterTitle string, metadata *ChapterMetadata, registry Registry) *ComicInfo {
	pages := make([]ComicInfoPage, 0, len(registry))
	for _, pageInfo := range registry {
		pages = append(pages, ComicInfoPage{
			Image:       pageInfo.Index,
			ImageSize:   pageInfo.Size,
			ImageWidth:  pageInfo.Width,
			ImageHeight: pageInfo.Height,
		})
	}

	info := NewComicInfo()
	info.Title = chapterTitle
	info.Series = mediaTitle
	info.Number = id.ChapterNumber
	if metadata != nil {
		info.Summary = metadata.Summary
		info.Notes = metadata.Notes
		info.Year = metadata.Year
		info.Count = metadata.Count
		info.Writer = metadata.Writer
		info.Penciller = metadata.Penciller
		info.Translator = metadata.Translator
		info.Genre = metadata.Genre
		info.Tags = metadata.Tags
		info.Web = metadata.Web
		info.LanguageISO = metadata.LanguageISO
		info.Characters = metadata.Characters
		info.AgeRating = metadata.AgeRating
		// Only chapters Seanime scraped from a manga source carry metadata, so
		// the reading direction is known to be manga's.
		info.Manga = MangaRightToLeft
	}
	info.SetPages(pages)

	return info
}

// Marshal renders the document as an XML file, header included.
func (ci *ComicInfo) Marshal() ([]byte, error) {
	data, err := xml.MarshalIndent(ci, "", "  ")
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), data...), nil
}

// ParseComicInfo reads a ComicInfo document, ignoring elements it does not model.
func ParseComicInfo(r io.Reader) (*ComicInfo, error) {
	var ci ComicInfo
	if err := xml.NewDecoder(r).Decode(&ci); err != nil {
		return nil, err
	}
	return &ci, nil
}
