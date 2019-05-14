package wails

import (
	"encoding/json"
	"fmt"

	"github.com/leaanthony/mewn"
	"github.com/raff/ultralight-go"
)

// Window defines the main application window
// Default values in []
type ultralightRenderer struct {
        app          *ultralight.App
	window       *ultralight.Window
	ipc          *ipcManager
	log          *CustomLogger
	config       *AppConfig
	eventManager *eventManager
	bindingCache []string
	frameworkJS  string
	frameworkCSS string

	// This is a list of all the JS/CSS that needs injecting
	// It will get injected in order
	jsCache  []string
	cssCache []string
}

// Initialise sets up the Ultralight renderer
func (w *ultralightRenderer) Initialise(config *AppConfig, ipc *ipcManager, eventManager *eventManager) error {

	// Store reference to eventManager
	w.eventManager = eventManager

	// Set up logger
	w.log = newCustomLogger("Ultralight")

	// Set up the dispatcher function
	w.ipc = ipc
	ipc.bindRenderer(w)

	// Save the config
	w.config = config

	// Create the Ultralight instance
        w.app = ultralight.NewApp()
        w.window = w.app.NewWindow(
                    uint(config.Width),
                    uint(config.Height),
                    false, // fullscreen
                    config.Title)

        w.window.View().LoadURL(config.defaultHTML)

	// SignalManager.OnExit(w.Exit)

	// Set colour
	err := w.SetColour(config.Colour)
	if err != nil {
		return err
	}

	w.log.Info("Initialised")
	return nil
}

func (w *ultralightRenderer) SetColour(colour string) error {
        /*
	color, err := colors.Parse(colour)
	if err != nil {
		return err
	}
	rgba := color.ToRGBA()
	alpha := uint8(255 * rgba.A)
	w.window.Dispatch(func() {
		w.window.SetColor(rgba.R, rgba.G, rgba.B, alpha)
	})
        */

	return nil
}

// evalJS evaluates the given js in the current view
// I should rename this to evilJS lol
func (w *ultralightRenderer) evalJS(js string) error {
	outputJS := fmt.Sprintf("%.45s", js)
	if len(js) > 45 {
		outputJS += "..."
	}
	w.log.DebugFields("Eval", Fields{"js": outputJS})
	//
        w.window.View().EvaluateScript(js)
	return nil
}

// evalJSSync evaluates the given js in the current view synchronously
// Do not call this from the main thread or you'll nuke your app because
// you won't get the callback.
func (w *ultralightRenderer) evalJSSync(js string) error {

	//minified, err := escapeJS(js)
	//if err != nil {
	//	return err
	//}

	outputJS := fmt.Sprintf("%.45s", js)
	if len(js) > 45 {
		outputJS += "..."
	}
	w.log.DebugFields("EvalSync", Fields{"js": outputJS})
        w.window.View().EvaluateScript(js)
/*
	ID := fmt.Sprintf("syncjs:%d:%d", time.Now().Unix(), rand.Intn(9999))
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		exit := false
		// We are done when we receive the Callback ID
		w.log.Debug("SyncJS: sending with ID = " + ID)
		w.eventManager.On(ID, func(...interface{}) {
			w.log.Debug("SyncJS: Got callback ID = " + ID)
			wg.Done()
			exit = true
		})
		command := fmt.Sprintf("wails._.addScript('%s', '%s')", minified, ID)
		w.window.Dispatch(func() {
			w.window.Eval(command)
		})
		for exit == false {
			time.Sleep(time.Millisecond * 1)
		}
	}()

	wg.Wait()
*/
	return nil
}

// injectCSS adds the given CSS to the current view
func (w *ultralightRenderer) injectCSS(css string) {
	//w.window.Dispatch(func() {
	//	w.window.InjectCSS(css)
	//})
}

// Quit the window
func (w *ultralightRenderer) Exit() {
	w.window.Close()
}

// Run the window main loop
func (w *ultralightRenderer) Run() error {

	w.log.Info("Run()")

	// Runtime assets
	wailsRuntime := mewn.String("./wailsruntimeassets/default/wails.min.js")
	w.evalJS(wailsRuntime)

	// Ping the wait channel when the wails runtime is loaded
	w.eventManager.On("wails:loaded", func(...interface{}) {

		// Run this in a different go routine to free up the main process
		go func() {

			// Inject Bindings
			for _, binding := range w.bindingCache {
				w.evalJSSync(binding)
			}

			// Inject Framework
			if w.frameworkJS != "" {
				w.evalJSSync(w.frameworkJS)
			}
			if w.frameworkCSS != "" {
				w.injectCSS(w.frameworkCSS)
			}

			// Inject user CSS
			if w.config.CSS != "" {
				outputCSS := fmt.Sprintf("%.45s", w.config.CSS)
				if len(outputCSS) > 45 {
					outputCSS += "..."
				}
				w.log.DebugFields("Inject User CSS", Fields{"css": outputCSS})
				w.injectCSS(w.config.CSS)
			} else {
				// Use default wails css
				w.log.Debug("Injecting Default Wails CSS")
				defaultCSS := mewn.String("./wailsruntimeassets/default/wails.css")

				w.injectCSS(defaultCSS)
			}

			// Inject all the CSS files that have been added
			for _, css := range w.cssCache {
				w.injectCSS(css)
			}

			// Inject all the JS files that have been added
			for _, js := range w.jsCache {
				w.evalJSSync(js)
			}

			// Inject user JS
			if w.config.JS != "" {
				outputJS := fmt.Sprintf("%.45s", w.config.JS)
				if len(outputJS) > 45 {
					outputJS += "..."
				}
				w.log.DebugFields("Inject User JS", Fields{"js": outputJS})
				w.evalJSSync(w.config.JS)
			}

			// Emit that everything is loaded and ready
			w.eventManager.Emit("wails:ready")
		}()
	})

	// Kick off main window loop
	w.app.Run()

	return nil
}

// Binds the given method name with the front end
func (w *ultralightRenderer) NewBinding(methodName string) error {
	objectCode := fmt.Sprintf("window.wails._.newBinding('%s');", methodName)
	w.bindingCache = append(w.bindingCache, objectCode)
	return nil
}

func (w *ultralightRenderer) SelectFile() string {
	var result string

	// We need to run this on the main thread, however Dispatch is
	// non-blocking so we launch this in a goroutine and wait for
	// dispatch to finish before returning the result
        /*
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		w.window.Dispatch(func() {
			result = w.window.Dialog(webview.DialogTypeOpen, 0, "Select File", "")
			wg.Done()
		})
	}()
	wg.Wait()
        */
	return result
}

func (w *ultralightRenderer) SelectDirectory() string {
	var result string
	// We need to run this on the main thread, however Dispatch is
	// non-blocking so we launch this in a goroutine and wait for
	// dispatch to finish before returning the result
        /*
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		w.window.Dispatch(func() {
			result = w.window.Dialog(webview.DialogTypeOpen, webview.DialogFlagDirectory, "Select Directory", "")
			wg.Done()
		})
	}()
	wg.Wait()
        */
	return result
}

func (w *ultralightRenderer) SelectSaveFile() string {
	var result string
	// We need to run this on the main thread, however Dispatch is
	// non-blocking so we launch this in a goroutine and wait for
	// dispatch to finish before returning the result
        /*
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		w.window.Dispatch(func() {
			result = w.window.Dialog(webview.DialogTypeSave, 0, "Save file", "")
			wg.Done()
		})
	}()
	wg.Wait()
        */
	return result
}

// AddJS adds a piece of Javascript to a cache that
// gets injected at runtime
func (w *ultralightRenderer) AddJSList(jsCache []string) {
	w.jsCache = jsCache
}

// AddCSSList sets the cssCache to the given list of strings
func (w *ultralightRenderer) AddCSSList(cssCache []string) {
	w.cssCache = cssCache
}

// Callback sends a callback to the frontend
func (w *ultralightRenderer) Callback(data string) error {
	callbackCMD := fmt.Sprintf("window.wails._.callback('%s');", data)
	return w.evalJS(callbackCMD)
}

func (w *ultralightRenderer) NotifyEvent(event *eventData) error {

	// Look out! Nils about!
	var err error
	if event == nil {
		err = fmt.Errorf("Sent nil event to renderer.ultralightRenderer")
		logger.Error(err)
		return err
	}

	// Default data is a blank array
	data := []byte("[]")

	// Process event data
	if event.Data != nil {
		// Marshall the data
		data, err = json.Marshal(event.Data)
		if err != nil {
			w.log.Errorf("Cannot unmarshall JSON data in event: %s ", err.Error())
			return err
		}
	}

	message := fmt.Sprintf("wails._.notify('%s','%s')", event.Name, data)
	return w.evalJS(message)
}

// Window
func (w *ultralightRenderer) Fullscreen() {
	if w.config.Resizable == false {
		w.log.Warn("Cannot call Fullscreen() - App.Resizable = false")
		return
	}
	//w.window.Dispatch(func() {
        //	w.window.SetFullscreen(true)
	//})
}

func (w *ultralightRenderer) UnFullscreen() {
	if w.config.Resizable == false {
		w.log.Warn("Cannot call UnFullscreen() - App.Resizable = false")
		return
	}
	//w.window.Dispatch(func() {
	//	w.window.SetFullscreen(false)
	//})
}

func (w *ultralightRenderer) SetTitle(title string) {
        w.window.SetTitle(title)
}

func (w *ultralightRenderer) Close() {
	w.window.Close()
}
