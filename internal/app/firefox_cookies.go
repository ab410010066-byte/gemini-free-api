package app

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	_ "modernc.org/sqlite"
)

// FirefoxContainerAccount 代表 Firefox 某一个容器中的 Google 登录账号
type FirefoxContainerAccount struct {
	UserContextID int
	ContainerName string
	CookieString  string
	CookieCount   int
	SAPISID       string
}

// findFirefoxProfileDir 寻找本地 Firefox 配置目录实体
func findFirefoxProfileDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	var ffDirs []string
	switch runtime.GOOS {
	case "darwin":
		ffDirs = []string{
			filepath.Join(home, "Library", "Application Support", "Firefox"),
		}
	case "windows":
		appdata := os.Getenv("APPDATA")
		if appdata == "" {
			appdata = filepath.Join(home, "AppData", "Roaming")
		}
		ffDirs = []string{
			filepath.Join(appdata, "Mozilla", "Firefox"),
		}
	default:
		ffDirs = []string{
			filepath.Join(home, ".mozilla", "firefox"),
		}
	}

	for _, ffDir := range ffDirs {
		// 1. 优先解析 profiles.ini 锁定当前正在使用的 Default Profile
		iniPath := filepath.Join(ffDir, "profiles.ini")
		if f, err := os.Open(iniPath); err == nil {
			var candidates []string
			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if strings.HasPrefix(line, "Path=") {
					p := strings.TrimPrefix(line, "Path=")
					candidates = append(candidates, filepath.Join(ffDir, p))
				} else if strings.HasPrefix(line, "Default=") {
					p := strings.TrimPrefix(line, "Default=")
					candidates = append([]string{filepath.Join(ffDir, p)}, candidates...)
				}
			}
			f.Close()
			for _, c := range candidates {
				if fi, err := os.Stat(c); err == nil && fi.IsDir() {
					return c, nil
				}
			}
		}

		// 2. 扫描 Profiles 目录实体
		profilesDir := filepath.Join(ffDir, "Profiles")
		if entries, err := os.ReadDir(profilesDir); err == nil {
			var releaseCandidates []string
			var otherCandidates []string
			for _, e := range entries {
				if !e.IsDir() {
					continue
				}
				pDir := filepath.Join(profilesDir, e.Name())
				if strings.Contains(e.Name(), "default-release") {
					releaseCandidates = append(releaseCandidates, pDir)
				} else {
					otherCandidates = append(otherCandidates, pDir)
				}
			}
			if len(releaseCandidates) > 0 {
				return releaseCandidates[0], nil
			}
			if len(otherCandidates) > 0 {
				return otherCandidates[0], nil
			}
		}
	}

	return "", fmt.Errorf("未找到本地 Firefox 配置文件目录")
}

// findFirefoxCookiesDB 寻找本地 Firefox 配置目录中的 cookies.sqlite
func findFirefoxCookiesDB() (string, error) {
	if custom := os.Getenv("FIREFOX_COOKIES_DB"); custom != "" {
		if fi, err := os.Stat(custom); err == nil && !fi.IsDir() && fi.Size() > 0 {
			return custom, nil
		}
	}

	// 优先检查本地 data 目录（免疫 macOS launchd TCC 隐私限制）
	localCandidates := []string{
		filepath.Join("data", "firefox_cookies.sqlite"),
		filepath.Join(".", "firefox_cookies.sqlite"),
	}
	for _, lc := range localCandidates {
		if fi, err := os.Stat(lc); err == nil && !fi.IsDir() && fi.Size() > 0 {
			logf("[firefox] using local data cookie DB: %s", lc)
			return lc, nil
		}
	}

	if profDir, err := findFirefoxProfileDir(); err == nil {
		cPath := filepath.Join(profDir, "cookies.sqlite")
		if fi, err := os.Stat(cPath); err == nil && !fi.IsDir() && fi.Size() > 0 {
			return cPath, nil
		}
	}

	return "", fmt.Errorf("未找到本地 Firefox 配置文件目录，请确认 Firefox 是否安装且有使用记录")
}

// copyFile 复制文件辅助函数
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

// loadFirefoxContainerNames 从 containers.json 读取容器名称映射 (userContextId -> name)
func loadFirefoxContainerNames(profileDir string) map[int]string {
	names := map[int]string{}
	cPath := filepath.Join(profileDir, "containers.json")
	data, err := os.ReadFile(cPath)
	if err != nil {
		return names
	}
	var cj struct {
		Identities []struct {
			UserContextID int    `json:"userContextId"`
			Name          string `json:"name"`
			Public        bool   `json:"public"`
		} `json:"identities"`
	}
	if err := json.Unmarshal(data, &cj); err != nil {
		return names
	}
	for _, id := range cj.Identities {
		if id.Name != "" && id.Public {
			names[id.UserContextID] = id.Name
		}
	}
	return names
}

// readFirefoxGoogleCookies 读取所有容器（包含主容器与各 Multi-Account Containers）中的 Google 账号
func readFirefoxGoogleCookies() ([]FirefoxContainerAccount, error) {
	dbPath, err := findFirefoxCookiesDB()
	if err != nil {
		return nil, err
	}
	profDir := filepath.Dir(dbPath)
	containerNames := loadFirefoxContainerNames(profDir)

	// 复制到临时文件以防锁定正在运行的 Firefox
	tmpDir := os.TempDir()
	tmpDB := filepath.Join(tmpDir, fmt.Sprintf("ff_cookies_%d.sqlite", os.Getpid()))
	defer os.Remove(tmpDB)
	defer os.Remove(tmpDB + "-wal")
	defer os.Remove(tmpDB + "-shm")

	if err := copyFile(dbPath, tmpDB); err != nil {
		return nil, fmt.Errorf("读取 Firefox 数据库失败: %w", err)
	}
	if err := copyFile(dbPath+"-wal", tmpDB+"-wal"); err != nil && !os.IsNotExist(err) {
	}
	if err := copyFile(dbPath+"-shm", tmpDB+"-shm"); err != nil && !os.IsNotExist(err) {
	}

	db, err := sql.Open("sqlite", tmpDB)
	if err != nil {
		return nil, fmt.Errorf("打开 Firefox 数据库失败: %w", err)
	}
	defer db.Close()

	rows, err := db.Query(`
		SELECT originAttributes, name, value FROM moz_cookies 
		WHERE host LIKE '%google.com' 
		ORDER BY lastAccessed DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("查询 Firefox Cookies 失败: %w", err)
	}
	defer rows.Close()

	// 按 originAttributes 分组
	groups := make(map[string]map[string]string)
	for rows.Next() {
		var attr, name, value string
		if err := rows.Scan(&attr, &name, &value); err == nil && value != "" {
			if groups[attr] == nil {
				groups[attr] = make(map[string]string)
			}
			if _, exists := groups[attr][name]; !exists {
				groups[attr][name] = value
			}
		}
	}

	cidRe := regexp.MustCompile(`userContextId=(\d+)`)
	var accounts []FirefoxContainerAccount

	for attr, cmap := range groups {
		sapisid, ok := cmap["SAPISID"]
		psid, hasPsid := cmap["__Secure-1PSID"]
		if !ok || sapisid == "" || !hasPsid || psid == "" {
			continue // 未完整登录 Google 的容器跳过（必须包含 SAPISID 和 __Secure-1PSID）
		}
		cid := 0
		if m := cidRe.FindStringSubmatch(attr); len(m) > 1 {
			cid, _ = strconv.Atoi(m[1])
		}
		name := "Firefox 主環境 (Default)"
		if cid > 0 {
			if cName, exists := containerNames[cid]; exists && cName != "" {
				name = cName
			} else {
				name = fmt.Sprintf("container-%d", cid)
			}
		}

		var parts []string
		for k, v := range cmap {
			parts = append(parts, fmt.Sprintf("%s=%s", k, v))
		}

		accounts = append(accounts, FirefoxContainerAccount{
			UserContextID: cid,
			ContainerName: name,
			CookieString:  strings.Join(parts, "; "),
			CookieCount:   len(cmap),
			SAPISID:       sapisid,
		})
	}

	if len(accounts) == 0 {
		return nil, fmt.Errorf("Firefox 中未检测到有效 Google 登录凭证（缺少 SAPISID），请先在 Firefox 或容器中登录 gemini.google.com")
	}

	return accounts, nil
}

// SyncAllFirefoxContainers 从本地 Firefox 批量同步所有 Google 容器账号
func SyncAllFirefoxContainers() (createdCount, updatedCount int, details []string, err error) {
	accounts, err := readFirefoxGoogleCookies()
	if err != nil {
		return 0, 0, nil, err
	}

	for _, a := range accounts {
		note := fmt.Sprintf("Firefox Multi-Account Containers [ID:%d, Name:%s] (%d cookies)", a.UserContextID, a.ContainerName, a.CookieCount)
		id, updated, err := accountUpsert(a.ContainerName, a.CookieString, note)
		if err != nil {
			logf("[cookies] Firefox 同步账号 %s 失败: %v", a.ContainerName, err)
			continue
		}
		if updated {
			updatedCount++
			details = append(details, fmt.Sprintf("#%d %s (更新)", id, a.ContainerName))
		} else {
			createdCount++
			details = append(details, fmt.Sprintf("#%d %s (新增)", id, a.ContainerName))
		}
	}
	return createdCount, updatedCount, details, nil
}

// handleAdminFirefoxImport 一键从本地 Firefox Multi-Account Containers 批量导入 Google 账号
func handleAdminFirefoxImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}

	createdCount, updatedCount, results, err := SyncAllFirefoxContainers()
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}

	logf("[cookies] Firefox 一键同步完成：新增 %d 个，更新 %d 个", createdCount, updatedCount)
	writeJSON(w, 200, map[string]interface{}{
		"ok":      true,
		"total":   len(results),
		"details": results,
		"message": fmt.Sprintf("已成功从 Firefox Multi-Account Containers 同步 %d 个账号：%s", len(results), strings.Join(results, ", ")),
	})
}

// FirefoxIdentityItem 对应 containers.json 里的一个容器
type FirefoxIdentityItem struct {
	UserContextID int    `json:"userContextId"`
	Public        bool   `json:"public"`
	Icon          string `json:"icon"`
	Color         string `json:"color"`
	Name          string `json:"name"`
	AccessKey     string `json:"accessKey,omitempty"`
}

type FirefoxContainersFile struct {
	Version           int                   `json:"version"`
	LastUserContextID int                   `json:"lastUserContextId"`
	Identities        []FirefoxIdentityItem `json:"identities"`
	SiteAssociations  map[string]interface{} `json:"siteAssociations"`
}

// AutoCreateFirefoxContainers 自动在本地 Firefox profile 的 containers.json 中批量创建隔离容器
func AutoCreateFirefoxContainers(count int, prefix string) ([]string, error) {
	if count <= 0 {
		count = 5
	}
	if count > 50 {
		count = 50
	}
	if prefix == "" {
		prefix = "Gemini"
	}

	profDir, err := findFirefoxProfileDir()
	if err != nil {
		dbPath, dberr := findFirefoxCookiesDB()
		if dberr != nil {
			return nil, fmt.Errorf("找不到本地 Firefox 配置目录: %w", err)
		}
		profDir = filepath.Dir(dbPath)
	}
	cPath := filepath.Join(profDir, "containers.json")

	var cf FirefoxContainersFile
	data, err := os.ReadFile(cPath)
	if err == nil {
		_ = json.Unmarshal(data, &cf)
	}
	if cf.Version == 0 {
		cf.Version = 8
	}
	if cf.SiteAssociations == nil {
		cf.SiteAssociations = make(map[string]interface{})
	}

	maxID := cf.LastUserContextID
	for _, id := range cf.Identities {
		if id.UserContextID > maxID && id.UserContextID < 1000000 {
			maxID = id.UserContextID
		}
	}
	if maxID < 10 {
		maxID = 10
	}

	colors := []string{"blue", "turquoise", "green", "yellow", "orange", "red", "pink", "purple"}
	icons := []string{"circle", "fingerprint", "briefcase", "dollar", "cart", "gift", "tree", "chill"}

	var created []string
	for i := 1; i <= count; i++ {
		cid := maxID + i
		cName := fmt.Sprintf("%s-%02d", prefix, cid)
		color := colors[(cid)%len(colors)]
		icon := icons[(cid)%len(icons)]

		cf.Identities = append(cf.Identities, FirefoxIdentityItem{
			UserContextID: cid,
			Public:        true,
			Icon:          icon,
			Color:         color,
			Name:          cName,
		})
		created = append(created, cName)
	}
	cf.LastUserContextID = maxID + count

	outData, err := json.MarshalIndent(cf, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("生成 containers.json 失败: %w", err)
	}

	if err := os.WriteFile(cPath, outData, 0644); err != nil {
		return nil, fmt.Errorf("写入 containers.json 失败: %w", err)
	}

	return created, nil
}

// GetFirefoxContainersStatus 返回当前本地 Firefox 容器列表及其 Google 登录态探针
func GetFirefoxContainersStatus() (map[string]interface{}, error) {
	profDir, err := findFirefoxProfileDir()
	if err != nil {
		dbPath, dberr := findFirefoxCookiesDB()
		if dberr != nil {
			return nil, fmt.Errorf("未检测到本地 Firefox: %w", err)
		}
		profDir = filepath.Dir(dbPath)
	}
	cPath := filepath.Join(profDir, "containers.json")

	dbPath, err := findFirefoxCookiesDB()
	if err != nil {
		return nil, fmt.Errorf("未检测到本地 Firefox Cookie 数据库: %w", err)
	}

	var cf FirefoxContainersFile
	data, err := os.ReadFile(cPath)
	if err == nil {
		_ = json.Unmarshal(data, &cf)
	}

	// 临时拷贝 cookies.sqlite 读取登录态
	tmpDir := os.TempDir()
	tmpDB := filepath.Join(tmpDir, fmt.Sprintf("ff_status_%d.sqlite", os.Getpid()))
	defer os.Remove(tmpDB)
	defer os.Remove(tmpDB + "-wal")
	defer os.Remove(tmpDB + "-shm")

	loggedInCids := make(map[int]int)
	if err := copyFile(dbPath, tmpDB); err == nil {
		_ = copyFile(dbPath+"-wal", tmpDB+"-wal")
		_ = copyFile(dbPath+"-shm", tmpDB+"-shm")
		if db, err := sql.Open("sqlite", tmpDB); err == nil {
			defer db.Close()
			rows, qerr := db.Query(`SELECT originAttributes, count(*) FROM moz_cookies WHERE (name='SAPISID' OR name='__Secure-1PSID') AND value != '' GROUP BY originAttributes`)
			if qerr == nil {
				cidRe := regexp.MustCompile(`userContextId=(\d+)`)
				for rows.Next() {
					var attr string
					var cnt int
					if err := rows.Scan(&attr, &cnt); err == nil {
						if m := cidRe.FindStringSubmatch(attr); len(m) > 1 {
							if cid, err := strconv.Atoi(m[1]); err == nil && cid > 0 {
								loggedInCids[cid] = cnt
							}
						}
					}
				}
				rows.Close()
			}
		}
	}

	type containerStatusView struct {
		ID       int    `json:"id"`
		Name     string `json:"name"`
		Color    string `json:"color"`
		Icon     string `json:"icon"`
		LoggedIn bool   `json:"logged_in"`
		Cookies  int    `json:"cookie_count"`
	}

	var list []containerStatusView
	for _, id := range cf.Identities {
		if id.Public && id.Name != "" {
			cnt := loggedInCids[id.UserContextID]
			list = append(list, containerStatusView{
				ID:       id.UserContextID,
				Name:     id.Name,
				Color:    id.Color,
				Icon:     id.Icon,
				LoggedIn: cnt > 0,
				Cookies:  cnt,
			})
		}
	}

	return map[string]interface{}{
		"ok":         true,
		"profile":    filepath.Base(profDir),
		"containers": list,
		"total":      len(list),
	}, nil
}

// handleAdminFirefoxCreateContainers 接口：POST /admin/api/firefox/create-containers
func handleAdminFirefoxCreateContainers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}

	var p struct {
		Count  int    `json:"count"`
		Prefix string `json:"prefix"`
	}
	_ = json.NewDecoder(r.Body).Decode(&p)
	if p.Count <= 0 {
		p.Count = 5
	}
	if p.Prefix == "" {
		p.Prefix = "Gemini-Fleet"
	}

	created, err := AutoCreateFirefoxContainers(p.Count, p.Prefix)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, 200, map[string]interface{}{
		"ok":      true,
		"created": created,
		"count":   len(created),
		"message": fmt.Sprintf("已成功在 Firefox 中自動建立 %d 個全新隔離容器：%s", len(created), strings.Join(created, ", ")),
	})
}

// handleAdminFirefoxStatus 接口：GET /admin/api/firefox/status
func handleAdminFirefoxStatus(w http.ResponseWriter, r *http.Request) {
	status, err := GetFirefoxContainersStatus()
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, status)
}
