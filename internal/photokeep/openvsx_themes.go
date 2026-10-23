package photokeep

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const openVSXBase = "https://open-vsx.org"

var openVSXIdentifier = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+-]{0,99}$`)
var openVSXHash = regexp.MustCompile(`\b[a-fA-F0-9]{64}\b`)
var openVSXInclude = regexp.MustCompile(`"include"\s*:\s*("(?:[^"\\]|\\.)*")`)

type openVSXExtension struct {
	Namespace   string `json:"namespace"`
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Files       struct {
		Download string `json:"download"`
		SHA256   string `json:"sha256"`
	} `json:"files"`
}

type OpenVSXThemeResult struct {
	Namespace   string `json:"namespace"`
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Description string `json:"description"`
	Version     string `json:"version"`
}

type OpenVSXThemeSearch struct {
	Extensions []OpenVSXThemeResult `json:"extensions"`
	Total      int                  `json:"total"`
	Offset     int                  `json:"offset"`
	NextOffset int                  `json:"nextOffset"`
	HasMore    bool                 `json:"hasMore"`
}

type OpenVSXThemePackage struct {
	Namespace string            `json:"namespace"`
	Name      string            `json:"name"`
	Version   string            `json:"version"`
	SHA256    string            `json:"sha256"`
	Files     map[string]string `json:"files"`
}

var openVSXHTTP = &http.Client{
	Timeout: 30 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 3 || req.URL.Scheme != "https" || (req.URL.Hostname() != "open-vsx.org" && req.URL.Hostname() != "openvsx.eclipsecontent.org") {
			return errors.New("Open VSX redirect ไปยังที่อยู่ที่ไม่อนุญาต")
		}
		return nil
	},
}

func openVSXGet(ctx context.Context, rawURL string, maxBytes int64) ([]byte, error) {
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme != "https" || u.Hostname() != "open-vsx.org" || u.Port() != "" || !strings.HasPrefix(u.Path, "/api/") {
		return nil, errors.New("ที่อยู่ไฟล์ Open VSX ไม่ถูกต้อง")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json, application/octet-stream, text/plain")
	resp, err := openVSXHTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("เชื่อมต่อ Open VSX ไม่สำเร็จ: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Open VSX ตอบกลับ HTTP %d", resp.StatusCode)
	}
	if resp.ContentLength > maxBytes {
		return nil, errors.New("แพ็กเกจธีมมีขนาดใหญ่เกินที่อนุญาต")
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, errors.New("แพ็กเกจธีมมีขนาดใหญ่เกินที่อนุญาต")
	}
	return data, nil
}

func SearchOpenVSXThemes(query string, offset int) (OpenVSXThemeSearch, error) {
	query = strings.TrimSpace(query)
	if len(query) < 2 || len(query) > 100 {
		return OpenVSXThemeSearch{}, errors.New("กรุณาใส่คำค้น 2–100 ตัวอักษร")
	}
	if offset < 0 || offset > 5000 {
		return OpenVSXThemeSearch{}, errors.New("หน้าค้นหาไม่ถูกต้อง")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result := OpenVSXThemeSearch{Extensions: []OpenVSXThemeResult{}, Offset: offset, NextOffset: offset}
	for page := 0; page < 5 && len(result.Extensions) < 12; page++ {
		u := openVSXBase + "/api/-/search?query=" + url.QueryEscape(query) + "&category=Themes&size=12&offset=" + fmt.Sprint(result.NextOffset)
		data, err := openVSXGet(ctx, u, 2<<20)
		if err != nil {
			return OpenVSXThemeSearch{}, err
		}
		var response struct {
			Extensions []openVSXExtension `json:"extensions"`
			TotalSize  int                `json:"totalSize"`
		}
		if err := json.Unmarshal(data, &response); err != nil {
			return OpenVSXThemeSearch{}, fmt.Errorf("ข้อมูลค้นหาไม่ถูกต้อง: %w", err)
		}
		result.Total = response.TotalSize
		compatible := make([]bool, len(response.Extensions))
		var group sync.WaitGroup
		limit := make(chan struct{}, 6)
		for i, item := range response.Extensions {
			if !openVSXIdentifier.MatchString(item.Namespace) || !openVSXIdentifier.MatchString(item.Name) || !openVSXIdentifier.MatchString(item.Version) {
				continue
			}
			group.Add(1)
			go func(i int, item openVSXExtension) {
				defer group.Done()
				limit <- struct{}{}
				defer func() { <-limit }()
				manifestURL := openVSXBase + "/api/" + item.Namespace + "/" + item.Name + "/" + item.Version + "/file/package.json"
				manifest, err := openVSXGet(ctx, manifestURL, 1<<20)
				compatible[i] = err == nil && openVSXHasColorThemes(manifest)
			}(i, item)
		}
		group.Wait()
		for i, item := range response.Extensions {
			if compatible[i] {
				result.Extensions = append(result.Extensions, OpenVSXThemeResult{item.Namespace, item.Name, item.DisplayName, item.Description, item.Version})
			}
		}
		result.NextOffset += len(response.Extensions)
		if len(response.Extensions) == 0 || result.NextOffset >= result.Total {
			break
		}
	}
	result.HasMore = result.NextOffset < result.Total
	return result, nil
}

func openVSXHasColorThemes(data []byte) bool {
	var manifest struct {
		Contributes struct {
			Themes []struct {
				UITheme string `json:"uiTheme"`
				Path    string `json:"path"`
			} `json:"themes"`
		} `json:"contributes"`
	}
	if json.Unmarshal(data, &manifest) != nil {
		return false
	}
	for _, theme := range manifest.Contributes.Themes {
		if theme.Path != "" && (theme.UITheme == "vs" || theme.UITheme == "vs-dark" || theme.UITheme == "hc-black" || theme.UITheme == "hc-light") {
			return true
		}
	}
	return false
}

func FetchOpenVSXThemePackage(namespace, name, version string) (OpenVSXThemePackage, error) {
	if !openVSXIdentifier.MatchString(namespace) || !openVSXIdentifier.MatchString(name) || !openVSXIdentifier.MatchString(version) {
		return OpenVSXThemePackage{}, errors.New("ชื่อหรือเวอร์ชัน Extension ไม่ถูกต้อง")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	metadataURL := openVSXBase + "/api/" + namespace + "/" + name + "/" + version
	data, err := openVSXGet(ctx, metadataURL, 1<<20)
	if err != nil {
		return OpenVSXThemePackage{}, err
	}
	var meta openVSXExtension
	if err := json.Unmarshal(data, &meta); err != nil {
		return OpenVSXThemePackage{}, err
	}
	if meta.Namespace != namespace || meta.Name != name || meta.Version != version || meta.Files.Download == "" || meta.Files.SHA256 == "" {
		return OpenVSXThemePackage{}, errors.New("ข้อมูลแพ็กเกจ Open VSX ไม่ตรงกับรายการที่เลือก")
	}
	filePrefix := metadataURL + "/file/"
	if !strings.HasPrefix(meta.Files.Download, filePrefix) || !strings.HasPrefix(meta.Files.SHA256, filePrefix) {
		return OpenVSXThemePackage{}, errors.New("ลิงก์ดาวน์โหลดหรือ checksum ไม่ตรงกับแพ็กเกจที่เลือก")
	}
	hashFile, err := openVSXGet(ctx, meta.Files.SHA256, 1024)
	if err != nil {
		return OpenVSXThemePackage{}, err
	}
	expected := openVSXHash.FindString(string(hashFile))
	if expected == "" {
		return OpenVSXThemePackage{}, errors.New("ไม่พบ SHA-256 ของแพ็กเกจ")
	}
	vsix, err := openVSXGet(ctx, meta.Files.Download, 100<<20)
	if err != nil {
		return OpenVSXThemePackage{}, err
	}
	if err := verifyOpenVSXHash(vsix, expected); err != nil {
		return OpenVSXThemePackage{}, err
	}
	files, err := readOpenVSXThemeFiles(vsix)
	if err != nil {
		return OpenVSXThemePackage{}, err
	}
	return OpenVSXThemePackage{Namespace: namespace, Name: name, Version: version, SHA256: strings.ToLower(expected), Files: files}, nil
}

func verifyOpenVSXHash(data []byte, expected string) error {
	if !openVSXHash.MatchString(expected) || len(expected) != 64 {
		return errors.New("checksum SHA-256 ไม่ถูกต้อง")
	}
	actual := sha256.Sum256(data)
	if !strings.EqualFold(expected, hex.EncodeToString(actual[:])) {
		return errors.New("checksum SHA-256 ของแพ็กเกจไม่ตรงกัน")
	}
	return nil
}

func readOpenVSXThemeFiles(vsix []byte) (map[string]string, error) {
	archive, err := zip.NewReader(bytes.NewReader(vsix), int64(len(vsix)))
	if err != nil {
		return nil, errors.New("VSIX ไม่ใช่ไฟล์ ZIP ที่ถูกต้อง")
	}
	if len(archive.File) > 5000 {
		return nil, errors.New("VSIX มีไฟล์มากเกินไป")
	}
	entries := make(map[string]*zip.File)
	for _, file := range archive.File {
		name := file.Name
		if !strings.HasPrefix(name, "extension/") || path.Clean(name) != name || strings.Contains(name, "\\") || file.FileInfo().Mode()&os.ModeSymlink != 0 {
			continue
		}
		if !strings.HasSuffix(strings.ToLower(name), ".json") && !strings.HasSuffix(strings.ToLower(name), ".jsonc") {
			continue
		}
		entries[name] = file
	}
	files := make(map[string]string)
	var total int64
	var readFile func(string) error
	readFile = func(name string) error {
		if _, ok := files[name]; ok {
			return nil
		}
		file := entries[name]
		if file == nil {
			return fmt.Errorf("ไม่พบไฟล์ธีม %s", name)
		}
		if file.UncompressedSize64 > 8<<20 {
			return fmt.Errorf("ไฟล์ธีม %s มีขนาดใหญ่เกินไป", name)
		}
		if len(files) >= 512 {
			return errors.New("แพ็กเกจมีไฟล์ธีมมากเกินไป")
		}
		r, err := file.Open()
		if err != nil {
			return err
		}
		body, readErr := io.ReadAll(io.LimitReader(r, 8<<20+1))
		_ = r.Close()
		if readErr != nil {
			return readErr
		}
		if len(body) > 8<<20 {
			return errors.New("ไฟล์ธีมมีขนาดใหญ่เกินไป")
		}
		total += int64(len(body))
		if total > 128<<20 {
			return errors.New("ข้อมูลไฟล์สีที่ใช้งานมีขนาดใหญ่เกินไป")
		}
		files[name] = string(body)
		return nil
	}
	if err := readFile("extension/package.json"); err != nil {
		return nil, err
	}
	var manifest struct {
		Contributes struct {
			Themes []struct {
				Path string `json:"path"`
			} `json:"themes"`
		} `json:"contributes"`
	}
	if err := json.Unmarshal([]byte(files["extension/package.json"]), &manifest); err != nil {
		return nil, fmt.Errorf("package.json ไม่ถูกต้อง: %w", err)
	}
	if len(manifest.Contributes.Themes) == 0 {
		return nil, errors.New("แพ็กเกจไม่มีธีมสี")
	}
	var visit func(string, int) error
	visit = func(name string, depth int) error {
		if depth > 16 {
			return errors.New("ธีมอ้างอิงไฟล์ซ้อนมากเกินไป")
		}
		if err := readFile(name); err != nil {
			return err
		}
		match := openVSXInclude.FindStringSubmatch(files[name])
		if len(match) == 0 {
			return nil
		}
		include, err := strconv.Unquote(match[1])
		if err != nil {
			return err
		}
		base := path.Clean(path.Join(path.Dir(name), include))
		if !strings.HasPrefix(base, "extension/") || (base != "extension/package.json" && !strings.HasSuffix(strings.ToLower(base), ".json") && !strings.HasSuffix(strings.ToLower(base), ".jsonc")) {
			return errors.New("ตำแหน่งไฟล์ธีมไม่ปลอดภัย")
		}
		if _, ok := files[base]; ok {
			return nil
		}
		return visit(base, depth+1)
	}
	for _, theme := range manifest.Contributes.Themes {
		name := path.Clean(path.Join("extension", theme.Path))
		if !strings.HasPrefix(name, "extension/") || (!strings.HasSuffix(strings.ToLower(name), ".json") && !strings.HasSuffix(strings.ToLower(name), ".jsonc")) {
			return nil, errors.New("ตำแหน่งไฟล์ธีมไม่ปลอดภัย")
		}
		if err := visit(name, 0); err != nil {
			return nil, err
		}
	}
	return files, nil
}
