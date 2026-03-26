package browser

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

type Config struct {
	Headless  bool          `json:"headless"`
	Timeout   time.Duration `json:"timeout"`
	DataDir   string        `json:"data_dir"`   // cookie/session persistence
	UserAgent string        `json:"user_agent"`
}

func DefaultConfig() Config {
	return Config{
		Headless: true,
		Timeout:  30 * time.Second,
		DataDir:  "data/browser",
	}
}

type PageResult struct {
	Title string `json:"title"`
	URL   string `json:"url"`
	Text  string `json:"text,omitempty"`
}

// Engine wraps a Rod browser instance.
type Engine struct {
	mu             sync.Mutex
	cfg            Config
	browser        *rod.Browser
	page           *rod.Page         // active page
	screenshotHook func(data []byte) // called after every screenshot (JPEG bytes)
}

// SetScreenshotHook registers a callback invoked with raw screenshot bytes
// on each Screenshot/ScreenshotBytes call. Used for SSE streaming.
func (e *Engine) SetScreenshotHook(fn func([]byte)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.screenshotHook = fn
}

// ScreenshotBytes captures the current page as PNG and returns raw bytes.
func (e *Engine) ScreenshotBytes() ([]byte, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.page == nil {
		return nil, fmt.Errorf("browser: no active page")
	}

	data, err := e.page.Screenshot(true, nil)
	if err != nil {
		return nil, fmt.Errorf("browser: screenshot: %w", err)
	}

	// Fire hook if set (non-blocking)
	if e.screenshotHook != nil {
		go e.screenshotHook(data)
	}

	return data, nil
}

func New(cfg Config) (*Engine, error) {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}

	// ensure data dir
	if cfg.DataDir != "" {
		os.MkdirAll(cfg.DataDir, 0o755)
	}

	l := launcher.New()
	if cfg.Headless {
		l = l.Headless(true)
	} else {
		l = l.Headless(false)
	}
	if cfg.DataDir != "" {
		l = l.UserDataDir(filepath.Join(cfg.DataDir, "chrome-profile"))
	}

	u, err := l.Launch()
	if err != nil {
		return nil, fmt.Errorf("browser: launch: %w", err)
	}

	b := rod.New().ControlURL(u)
	if err := b.Connect(); err != nil {
		return nil, fmt.Errorf("browser: connect: %w", err)
	}

	slog.Info("browser: started", "headless", cfg.Headless)

	return &Engine{cfg: cfg, browser: b}, nil
}

// Navigate opens a URL and waits for the page to be ready.
func (e *Engine) Navigate(rawURL string) (*PageResult, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	page, err := e.browser.Page(proto.TargetCreateTarget{URL: rawURL})
	if err != nil {
		return nil, fmt.Errorf("browser: new page: %w", err)
	}
	page = page.Timeout(e.cfg.Timeout)

	if err := page.WaitStable(500 * time.Millisecond); err != nil {
		slog.Warn("browser: page did not stabilize, continuing", "err", err)
	}

	// close old page if exists
	if e.page != nil {
		e.page.Close()
	}
	e.page = page

	info, err := page.Info()
	if err != nil {
		return nil, fmt.Errorf("browser: page info: %w", err)
	}

	text, _ := page.Eval(`() => document.body ? document.body.innerText.substring(0, 5000) : ""`)
	preview := ""
	if text != nil {
		preview = text.Value.Str()
	}

	return &PageResult{
		Title: info.Title,
		URL:   info.URL,
		Text:  preview,
	}, nil
}

// Screenshot captures the current page as PNG and saves it to path.
func (e *Engine) Screenshot(savePath string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.page == nil {
		return fmt.Errorf("browser: no active page")
	}

	dir := filepath.Dir(savePath)
	os.MkdirAll(dir, 0o755)

	data, err := e.page.Screenshot(true, nil)
	if err != nil {
		return fmt.Errorf("browser: screenshot: %w", err)
	}

	// Fire hook if set (non-blocking)
	if e.screenshotHook != nil {
		go e.screenshotHook(data)
	}

	if err := os.WriteFile(savePath, data, 0o644); err != nil {
		return fmt.Errorf("browser: write screenshot: %w", err)
	}

	slog.Info("browser: screenshot saved", "path", savePath)
	return nil
}

// ReadText returns visible text from the page or a specific selector.
func (e *Engine) ReadText(selector string) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.page == nil {
		return "", fmt.Errorf("browser: no active page")
	}

	if selector == "" {
		result, err := e.page.Eval(`() => document.body ? document.body.innerText : ""`)
		if err != nil {
			return "", fmt.Errorf("browser: read body: %w", err)
		}
		return result.Value.Str(), nil
	}

	el, err := e.page.Element(selector)
	if err != nil {
		return "", fmt.Errorf("browser: element %q not found: %w", selector, err)
	}
	return el.Text()
}

// Click clicks an element matching the selector with a visual cursor animation.
func (e *Engine) Click(selector string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.page == nil {
		return fmt.Errorf("browser: no active page")
	}
	el, err := e.page.Element(selector)
	if err != nil {
		return fmt.Errorf("browser: element %q: %w", selector, err)
	}

	// Inject cursor animation before clicking
	shape, _ := el.Shape()
	if shape != nil && len(shape.Quads) > 0 {
		quad := shape.Quads[0]
		cx := (quad[0] + quad[2] + quad[4] + quad[6]) / 4
		cy := (quad[1] + quad[3] + quad[5] + quad[7]) / 4
		e.animateCursor(cx, cy)
	}

	return el.Click(proto.InputMouseButtonLeft, 1)
}

// animateCursor injects a temporary visual cursor at the target coordinates.
func (e *Engine) animateCursor(x, y float64) {
	if e.page == nil {
		return
	}
	js := fmt.Sprintf(`(function(){
		var c = document.createElement('div');
		c.id = '_yunque_cursor';
		c.style.cssText = 'position:fixed;z-index:2147483647;pointer-events:none;'
			+ 'width:24px;height:24px;border-radius:50%%;'
			+ 'background:rgba(59,130,246,0.5);border:2px solid #3b82f6;'
			+ 'box-shadow:0 0 12px rgba(59,130,246,0.4);'
			+ 'left:%fpx;top:%fpx;transform:translate(-50%%,-50%%);'
			+ 'transition:all 0.3s cubic-bezier(0.4,0,0.2,1);opacity:1;';
		document.body.appendChild(c);
		setTimeout(function(){
			c.style.width='16px';c.style.height='16px';
			c.style.background='rgba(59,130,246,0.8)';
		}, 100);
		setTimeout(function(){
			c.style.width='32px';c.style.height='32px';
			c.style.opacity='0';c.style.background='rgba(59,130,246,0.1)';
		}, 300);
		setTimeout(function(){c.remove()}, 700);
	})()`, x, y)
	e.page.Eval(js)
	time.Sleep(350 * time.Millisecond)
}

// Type types text into an input element.
func (e *Engine) Type(selector, text string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.page == nil {
		return fmt.Errorf("browser: no active page")
	}
	el, err := e.page.Element(selector)
	if err != nil {
		return fmt.Errorf("browser: element %q: %w", selector, err)
	}
	el.SelectAllText()
	return el.Input(text)
}

// Eval executes JavaScript in the page and returns the result as string.
func (e *Engine) Eval(js string) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.page == nil {
		return "", fmt.Errorf("browser: no active page")
	}
	result, err := e.page.Eval(js)
	if err != nil {
		return "", fmt.Errorf("browser: eval: %w", err)
	}
	return result.Value.String(), nil
}

// WaitFor waits for an element to appear.
func (e *Engine) WaitFor(ctx context.Context, selector string) error {
	e.mu.Lock()
	page := e.page
	e.mu.Unlock()

	if page == nil {
		return fmt.Errorf("browser: no active page")
	}
	_, err := page.Context(ctx).Element(selector)
	return err
}

// CurrentURL returns the current page URL.
func (e *Engine) CurrentURL() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.page == nil {
		return ""
	}
	info, err := e.page.Info()
	if err != nil {
		return ""
	}
	return info.URL
}

// Close shuts down the browser.
func (e *Engine) Close() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.browser != nil {
		e.browser.Close()
		slog.Info("browser: closed")
	}
}
