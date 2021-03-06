package main

import (
	"errors"
	"fmt"

	"github.com/codegangsta/cli"
	"github.com/codegangsta/envy/lib"
	"github.com/codegangsta/gin/lib"
	"github.com/everdev/mack"

	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var (
	startTime            = time.Now()
	logger               = log.New(os.Stdout, "[gingin] ", 0)
	immediate            = false
	notificationsEnabled = true
	buildError           error
)

func main() {
	app := cli.NewApp()
	app.Name = "gin"
	app.Usage = "A live reload utility for Go web applications."
	app.Action = MainAction
	app.Flags = []cli.Flag{
		cli.IntFlag{
			Name:  "port,p",
			Value: 3000,
			Usage: "port for the proxy server",
		},
		cli.IntFlag{
			Name:  "appPort,a",
			Value: 3001,
			Usage: "port for the Go web server",
		},
		cli.StringFlag{
			Name:  "bin,b",
			Value: "gin-bin",
			Usage: "name of generated binary file",
		},
		cli.StringFlag{
			Name:  "path,t",
			Value: ".",
			Usage: "Path to watch files from",
		},
		cli.IntFlag{
			Name:  "scanLower,sl",
			Value: 0,
			Usage: "Scan parent folders (useful for monorepos)",
		},
		cli.StringFlag{
			Name:  "exclude,e",
			Value: "",
			Usage: "Comma separated paths to ignore files in",
		},
		cli.StringFlag{
			Name:  "runArgs,u",
			Value: "",
			Usage: "Comma separated args to give to run",
		},
		cli.BoolFlag{
			Name:  "immediate,i",
			Usage: "run the server immediately after it's built",
		},
		cli.BoolFlag{
			Name:  "godep,g",
			Usage: "use godep when building",
		},
		cli.BoolFlag{
			Name:  "notify,n",
			Usage: "use OS X notifications when rebuilding",
		},
	}
	app.Commands = []cli.Command{
		{
			Name:      "run",
			ShortName: "r",
			Usage:     "Run the gin proxy in the current working directory",
			Action:    MainAction,
		},
		{
			Name:      "env",
			ShortName: "e",
			Usage:     "Display environment variables set by the .env file",
			Action:    EnvAction,
		},
	}

	app.Run(os.Args)
}

func MainAction(c *cli.Context) {
	port := c.GlobalInt("port")
	appPort := strconv.Itoa(c.GlobalInt("appPort"))
	immediate = c.GlobalBool("immediate")

	// Bootstrap the environment
	envy.Bootstrap()

	// Set the PORT env
	os.Setenv("PORT", appPort)

	wd, err := os.Getwd()
	if err != nil {
		logger.Fatal(err)
	}

	builder := gin.NewBuilder(c.GlobalString("path"), c.GlobalString("bin"), c.GlobalBool("godep"))

	// fmt.Println(c.Args())
	runArgs := strings.Split(c.GlobalString("runArgs"), ",")
	fmt.Println(c.GlobalString("runArgs"))

	// runArgs = append([]string{}, "-env", "development")
	runner := gin.NewRunner(filepath.Join(wd, builder.Binary()), runArgs...)
	runner.SetWriter(os.Stdout)
	proxy := gin.NewProxy(builder, runner)
	excludeList := strings.Split(c.GlobalString("exclude"), ",")

	config := &gin.Config{
		Port:    port,
		ProxyTo: "http://localhost:" + appPort,
	}

	err = proxy.Run(config)
	if err != nil {
		logger.Fatal(err)
	}

	logger.Printf("listening on port %d\n", port)

	shutdown(runner)

	// build right now
	build(builder, runner, logger)

	// scan for changes

	scanChanges(c.GlobalString("path"), c.GlobalInt("scanLower"), excludeList, func(path string) {
		runner.Kill()
		build(builder, runner, logger)
	})
}

func EnvAction(c *cli.Context) {
	// Bootstrap the environment
	env, err := envy.Bootstrap()
	if err != nil {
		logger.Fatalln(err)
	}

	for k, v := range env {
		fmt.Printf("%s: %s\n", k, v)
	}

}

func build(builder gin.Builder, runner gin.Runner, logger *log.Logger) {
	if err := builder.Build(); err != nil {
		buildError = err
		logger.Println("ERROR! Build failed.")
		logger.Println(builder.Errors())
	} else {
		// print success only if there were errors before, otherwise tell when rebuilt
		var notificationMessage string
		if buildError != nil {
			notificationMessage = "Build Successful"
			logger.Println(notificationMessage)
		} else {
			var curDir1 string
			curDir, _ := os.Getwd()
			segments := strings.Split(curDir, "/")
			if len(segments) > 0 {
				curDir1 = segments[len(segments)-1]
			}
			notificationMessage = fmt.Sprintf("%v - Rebuilt at %v \n", curDir1, time.Now().Format("15:04:05.999999"))
			logger.Printf(notificationMessage)
		}
		if notificationMessage != "" && notificationsEnabled {
			mack.Notify(notificationMessage)
		}
		buildError = nil
		if immediate {
			runner.Run()
		}
	}
	time.Sleep(100 * time.Millisecond)
}

type scanCallback func(path string)

func scanChanges(watchPath string, scanLower int, excludeList []string, cb scanCallback) {
	if scanLower > 0 {
		wtch, err := filepath.Abs(watchPath)
		if err == nil {
			watchPath = wtch
		}
		split := strings.Split(strings.TrimRight(watchPath, "/"), "/")
		if scanLower < len(split) {
			watchPath = strings.Join(split[0:len(split)-scanLower], "/")
		}
	}
	for {
		filepath.Walk(watchPath, func(path string, info os.FileInfo, err error) error {
			for _, ex := range excludeList {
				if path == ex {
					return filepath.SkipDir
				}
			}

			if path == ".git" {
				return filepath.SkipDir
			}

			// ignore hidden files
			if filepath.Base(path)[0] == '.' {
				return nil
			}

			if filepath.Ext(path) == ".go" && info.ModTime().After(startTime) {
				rebuildingString := "Detected changes, rebuilding..."
				logger.Println(rebuildingString)
				if notificationsEnabled {
					mack.Notify(rebuildingString)
				}
				cb(path)
				startTime = time.Now()
				return errors.New("done")
			}

			return nil
		})
		time.Sleep(500 * time.Millisecond)
	}
}

func shutdown(runner gin.Runner) {
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		s := <-c
		log.Println("Got signal: ", s)
		err := runner.Kill()
		if err != nil {
			log.Print("Error killing: ", err)
		}
		os.Exit(1)
	}()
}
