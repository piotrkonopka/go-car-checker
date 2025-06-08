package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	colly "github.com/gocolly/colly/v2"
)

type Segment string

// SS - SuperSuperb, S - Superb, A - Awesome, B - Better/Good, C - Common, D - Deficient, E - Error
const (
	SegmentSS Segment = "SS"
	SegmentS  Segment = "S"
	SegmentA  Segment = "A"
	SegmentB  Segment = "B"
	SegmentC  Segment = "C"
	SegmentD  Segment = "D"
	SegmentE  Segment = "E"
)

type SegmentConfig struct {
	Name       Segment
	MaxMileage int
	MaxPrice   int
	Prices     []int
	Matched    bool
}

func (seg *SegmentConfig) Accepts(mileage, price int) bool {
	return mileage <= seg.MaxMileage && price <= seg.MaxPrice
}

func (seg *SegmentConfig) UpdatePrice(price int) {
	seg.Prices = append(seg.Prices, price)
}

func classifySegment(mileage int, configs []*SegmentConfig) *SegmentConfig {
	for _, cfg := range configs {
		if mileage <= cfg.MaxMileage {
			return cfg
		}
	}
	return nil
}

func sendEmail(link string, price int, mileage int, segment Segment, brand string) {
	fmt.Printf("📧 [MAIL] [%s] Segment %s — %d zł, %d km — %s\n", brand, segment, price, mileage, link)
}

func saveCSV(brand string, segments []*SegmentConfig) {
	now := time.Now().Format("2006-01-02")
	fileName := fmt.Sprintf("data/%s.csv", strings.ReplaceAll(brand, " ", "_"))

	file, err := os.OpenFile(fileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("Błąd zapisu CSV: %v", err)
		return
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	for _, s := range segments {
		if len(s.Prices) > 0 {
			sum := 0
			for _, p := range s.Prices {
				sum += p
			}
			avg := sum / len(s.Prices)
			writer.Write([]string{now, string(s.Name), strconv.Itoa(avg)})
		}
	}
	fmt.Printf("✅ Wyniki zapisane do pliku %s\n", fileName)
}

// parsePrice extracts the integer price from a string containing a price (e.g., "45 000 zł").
// It removes spaces and non-breaking spaces, and supports both "zł" and "PLN" suffixes.
func parsePrice(text string) (int, error) {
	// Extract only the number before "zł"
	re := regexp.MustCompile(`([\d\s]+)zł`)
	match := re.FindStringSubmatch(text)
	if len(match) >= 2 {
		clean := strings.ReplaceAll(match[1], " ", "")
		clean = strings.ReplaceAll(clean, "\u00a0", "")
		return strconv.Atoi(clean)
	}
	// Fallback
	text = strings.ReplaceAll(text, "zł", "")
	text = strings.ReplaceAll(text, "PLN", "")
	text = strings.ReplaceAll(text, " ", "")
	text = strings.ReplaceAll(text, "\u00a0", "")
	return strconv.Atoi(text)
}

// parseMileage extracts the mileage from a string containing a mileage (e.g., "123 456 km").
// It removes spaces and non-breaking spaces, and expects the mileage to be followed by "km".
func parseMileage(text string) (int, error) {
	re := regexp.MustCompile(`([\d\s]+)\s*km`)
	match := re.FindStringSubmatch(text)
	if len(match) < 2 {
		return 0, fmt.Errorf("nie znaleziono przebiegu")
	}
	str := strings.ReplaceAll(match[1], " ", "")
	str = strings.ReplaceAll(str, "\u00a0", "")
	return strconv.Atoi(str)
}

func handleListing(price, mileage int, link, brand string, segments []*SegmentConfig) {
	segment := classifySegment(mileage, segments)
	if segment == nil {
		fmt.Println("❓ Nie udało się sklasyfikować segmentu")
		return
	}
	segment.UpdatePrice(price)
	if segment.Accepts(mileage, price) {
		// fmt.Printf("🎯 [%s] Segment %s — %d zł (limit %d), %d km\n", brand, segment.Name, price, segment.MaxPrice, mileage)
		if !segment.Matched {
			segment.Matched = true
			if segment.Name == SegmentB || segment.Name == SegmentC {
				sendEmail(link, price, mileage, segment.Name, brand)
			}
		}
	} else {
		// fmt.Printf("✘ [%s] Segment %s — %d zł (limit %d), %d km\n", brand, segment.Name, price, segment.MaxPrice, mileage)
	}
}

func addOLXCallback(c *colly.Collector, brand string, segments []*SegmentConfig) {
	c.OnHTML("div[data-cy='l-card']", func(e *colly.HTMLElement) {
		price, err := parsePrice(e.ChildText("p[data-testid='ad-price']"))
		if err != nil || price == 0 {
			fmt.Println("❌ Błąd parsowania ceny:", e.ChildText("p[data-testid='ad-price']"))
			return
		}

		var mileageText string
		e.ForEach("span", func(_ int, el *colly.HTMLElement) {
			if strings.Contains(el.Text, "km") && mileageText == "" {
				mileageText = el.Text
			}
		})
		if mileageText == "" {
			fmt.Println("❌ Nie znaleziono przebiegu.")
			return
		}
		mileage, err := parseMileage(mileageText)
		if err != nil {
			fmt.Println("❌ Błąd parsowania przebiegu:", mileageText)
			return
		}

		link := e.ChildAttr("a", "href")
		if !strings.HasPrefix(link, "http") {
			link = "https://www.olx.pl" + link
		}
		handleListing(price, mileage, link, brand, segments)
	})
}

func addOtomotoCallback(c *colly.Collector, brand string, segments []*SegmentConfig) {
	c.OnHTML("article[data-id]", func(e *colly.HTMLElement) {
		price, err := parsePrice(e.ChildText("h3"))
		if err != nil || price == 0 {
			fmt.Println("❌ Błąd parsowania ceny:", e.ChildText("h3"))
			return
		}

		mileageText := ""
		e.ForEach("dd", func(_ int, li *colly.HTMLElement) {
			if strings.Contains(li.Text, "km") && mileageText == "" {
				mileageText = li.Text
			}
		})
		if mileageText == "" {
			fmt.Println("❌ Nie znaleziono przebiegu.")
			return
		}
		mileage, err := parseMileage(mileageText)
		if err != nil {
			fmt.Println("❌ Błąd parsowania przebiegu:", mileageText)
			return
		}

		link := e.ChildAttr("a", "href")
		handleListing(price, mileage, link, brand, segments)
	})
}

func scanURL(url string, brand string) {
	fmt.Println("🌐 Skanuję:", brand)

	segments := []*SegmentConfig{
		{SegmentSS, 1000, 110000, nil, false},
		{SegmentS, 40000, 95000, nil, false},
		{SegmentA, 90000, 80000, nil, false},
		{SegmentB, 150000, 65000, nil, false},
		{SegmentC, 220000, 50000, nil, false},
		{SegmentD, 280000, 40000, nil, false},
		{SegmentE, 350000, 30000, nil, false},
	}

	c := colly.NewCollector(
		colly.AllowedDomains("www.otomoto.pl", "otomoto.pl", "www.olx.pl", "olx.pl"),
	)

	if strings.Contains(strings.ToLower(brand), "olx") {
		fmt.Println("🔍 Przetwarzam ogłoszenia z OLX")
		addOLXCallback(c, brand, segments)
	} else {
		fmt.Println("🔍 Przetwarzam ogłoszenia z Otomoto")
		addOtomotoCallback(c, brand, segments)
	}

	err := c.Visit(url)
	if err != nil {
		log.Printf("❌ Błąd podczas odwiedzania %s: %v", brand, err)
		return
	}

	fmt.Printf("\n📊 Podsumowanie [%s]:\n", brand)
	for _, s := range segments {
		if len(s.Prices) == 0 {
			fmt.Printf("Segment %s: brak ofert\n", s.Name)
			continue
		}
		min, max := s.Prices[0], s.Prices[0]
		for _, p := range s.Prices {
			if p < min {
				min = p
			}
			if p > max {
				max = p
			}
		}
		status := "❌ brak aut w limicie"
		if s.Matched {
			status = "✅ znaleziono auto w limicie"
		}
		fmt.Printf("Segment %s: min %d zł, max %d zł — %s\n", s.Name, min, max, status)
	}
	fmt.Println("✅ Zakończono skanowanie:", brand)
	fmt.Println("Zapisuję wyniki do pliku CSV...")
	saveCSV(brand, segments)
	fmt.Println("--------------------------------------------------")
}

func main() {
	urls := map[string]string{
		"Peugeot Rifter otomoto":     "https://www.otomoto.pl/osobowe/peugeot/rifter?search%5Bfilter_enum_damaged%5D=0&search%5Bfilter_enum_fuel_type%5D=diesel&search%5Bfilter_enum_gearbox%5D=automatic&search%5Bfilter_enum_has_vin%5D=1&search%5Bfilter_enum_no_accident%5D=1&search%5Border%5D=filter_float_price%3Aasc&search%5Badvanced_search_expanded%5D=true",
		"Citroen Berlingo otomoto":   "https://www.otomoto.pl/osobowe/citroen/berlingo?search%5Bfilter_enum_damaged%5D=0&search%5Bfilter_enum_fuel_type%5D=diesel&search%5Bfilter_enum_gearbox%5D=automatic&search%5Bfilter_enum_generation%5D=gen-iii-2018-berlingo&search%5Bfilter_enum_has_vin%5D=1&search%5Bfilter_enum_no_accident%5D=1&search%5Border%5D=filter_float_price%3Aasc&search%5Badvanced_search_expanded%5D=true",
		"Toyota Proace City otomoto": "https://www.otomoto.pl/osobowe/toyota/proace-city-verso?search%5Bfilter_enum_damaged%5D=0&search%5Bfilter_enum_fuel_type%5D=diesel&search%5Bfilter_enum_gearbox%5D=automatic&search%5Bfilter_enum_has_vin%5D=1&search%5Bfilter_enum_no_accident%5D=1&search%5Border%5D=filter_float_price%3Aasc&search%5Badvanced_search_expanded%5D=true",
		"Peugeot Rifter olx":         "https://www.olx.pl/motoryzacja/samochody/peugeot/?search%5Bphotos%5D=1&search%5Border%5D=filter_float_price:asc&search%5Bfilter_enum_model%5D%5B0%5D=rifter&search%5Bfilter_enum_petrol%5D%5B0%5D=diesel&search%5Bfilter_enum_condition%5D%5B0%5D=notdamaged&search%5Bfilter_enum_transmission%5D%5B0%5D=automatic",
		"Citroen Berlingo olx":       "https://www.olx.pl/motoryzacja/samochody/citroen/?search%5Border%5D=filter_float_price:asc&search%5Bfilter_enum_model%5D%5B0%5D=berlingo&search%5Bfilter_float_year:from%5D=2018&search%5Bfilter_enum_petrol%5D%5B0%5D=diesel&search%5Bfilter_enum_condition%5D%5B0%5D=notdamaged&search%5Bfilter_enum_transmission%5D%5B0%5D=automatic",
		"Toyota Proace City olx":     "https://www.olx.pl/motoryzacja/samochody/toyota/?search%5Border%5D=filter_float_price:asc&search%5Bfilter_enum_model%5D%5B0%5D=proace-city-verso&search%5Bfilter_enum_petrol%5D%5B0%5D=diesel&search%5Bfilter_enum_condition%5D%5B0%5D=notdamaged&search%5Bfilter_enum_transmission%5D%5B0%5D=automatic",
	}

	for brand, link := range urls {
		scanURL(link, brand)
	}
}
