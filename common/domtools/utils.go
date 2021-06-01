package domtools

import (
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/microcosm-cc/bluemonday"
	"github.com/pkg/errors"
	allris_common "github.com/rismaster/allris-common"
	"github.com/rismaster/allris-common/common/slog"
	"golang.org/x/net/html"
	"log"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func GetChildTextFromNode(node *html.Node) string {
	nodeDatas := GetAllNodeData(node, html.TextNode)
	return strings.TrimSpace(strings.Join(nodeDatas, " "))
}

func GetAllNodeData(node *html.Node, t html.NodeType) []string {
	if node.Type == t {
		return []string{CleanText(node.Data)}
	}
	var sum []string
	for c := node.FirstChild; c != nil; c = c.NextSibling {
		sum = append(sum, GetAllNodeData(c, t)...)
	}
	return sum
}

func CleanText(in string) string {
	const pattern = `<!--[\s\S]*?(?:-->)`
	m1 := regexp.MustCompile(pattern)
	in = m1.ReplaceAllString(in, " ")
	return strings.Join(strings.Fields(in), " ")
}

func GetAttrFromNode(node *html.Node, attrName string) string {
	for _, a := range node.Attr {
		if a.Key == attrName {
			return CleanText(a.Val)
		}
	}
	return ""
}

func ExtractIntFromInput(topTds *goquery.Selection, attrName string) int {
	inputVOLFDNR := topTds.Find("input[name=\"" + attrName + "\"]")
	var VOLFDNR = 0
	if inputVOLFDNR.Nodes != nil {
		VOLFDNRStr, _ := inputVOLFDNR.Attr("value")
		resultVo, err := strconv.Atoi(VOLFDNRStr)
		if err != nil {
			log.Printf("not a number %s", VOLFDNRStr)
			VOLFDNR = 0
		} else {
			VOLFDNR = resultVo
		}
	}
	return VOLFDNR
}

func ParseTable(descrNodes *goquery.Selection) ([]string, []string) {
	contNodes := descrNodes.NextFiltered("td")
	var bez []string
	var cont []string
	for _, tds := range descrNodes.Nodes {
		bez = append(bez, GetChildTextFromNode(tds))
	}
	for _, tds := range contNodes.Nodes {
		cont = append(cont, GetChildTextFromNode(tds))
	}
	return bez, cont
}

func FindIndex(bez []string, cont []string, s string) string {
	if len(cont) > len(bez) {
		slog.Error("nicht gleiche Länge %d != %d", len(cont), len(bez))
	}
	for index, elem := range bez {
		if elem == s {
			return cont[index]
		}
	}
	return ""
}

func FindIndexI(bez []string, cont []string, s string, i int) string {
	if len(cont) > len(bez) {
		slog.Error("nicht gleiche Länge %d != %d", len(cont), len(bez))
	}
	var cnt = 0
	for index, elem := range bez {
		if elem == s {
			cnt = cnt + 1
			if cnt == i {

				return cont[index]
			}
		}
	}
	return ""
}

func ExtractWeekdayDateFromCommaSeparated(datum string, hours string, config allris_common.Config) (time.Time, error) {
	datArr := strings.Split(datum, ",")
	if len(datArr) < 2 {
		return time.Time{}, errors.New("error extractWeekdayDateFromCommaSeparated from datum: " + datum)
	}

	zeitSplitted := strings.Split(hours, "-")
	location, err := time.LoadLocation(config.GetTimezone())
	if err != nil {
		return time.Time{}, err
	}
	date, err := time.ParseInLocation(config.GetDateFormatWithTime(), CleanText(fmt.Sprintf("%s %s:00", datArr[1], strings.TrimSpace(zeitSplitted[0]))), location)
	return date, err
}

func SanatizeHtml(html string, config allris_common.Config) string {
	p := bluemonday.NewPolicy()
	p.AllowStandardURLs()
	p.AllowAttrs("href").OnElements("a")
	p.AllowElements("a", "strong", "p", "b", "br", "pre", "h1", "h2", "h3", "h4", "h5", "h6", "h7", "table", "tr", "td", "ul", "li", "ol", "dd", "dt", "dl")
	san := p.Sanitize(html)
	const pattern = `<[^/>]+>[  \n\r\t]*</[^>]+>`
	m1 := regexp.MustCompile(pattern)
	san = m1.ReplaceAllString(san, " ")
	const pattern2 = `<a href=".*></a>`
	m12 := regexp.MustCompile(pattern2)
	san = m12.ReplaceAllString(san, " ")
	space := regexp.MustCompile(`[\s| ]+`)
	san = space.ReplaceAllString(san, " ")

	var re = regexp.MustCompile(`href="(.*?)"`)
	findings := re.FindAllString(san, -1)
	for i, f := range findings {
		if !strings.HasPrefix(f, "href=\"http") {
			elems := strings.Split(f, "\"")
			san = strings.ReplaceAll(san, f, fmt.Sprintf("href=\"%s\"", "https://"+path.Join(config.GetPathToParse(), elems[1])))
			log.Printf("%d %s, %s", i, f, fmt.Sprintf("href=\"%s\"", elems[1]))
		}
	}

	return strings.TrimSpace(san)
}

func StringToIntOrNeg(value string) int {
	tmp, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return tmp
}
