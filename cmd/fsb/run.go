package main

import (
	"EverythingSuckz/fsb/config"
	"EverythingSuckz/fsb/internal/bot"
	"EverythingSuckz/fsb/internal/cache"
	"EverythingSuckz/fsb/internal/routes"
	"EverythingSuckz/fsb/internal/types"
	"EverythingSuckz/fsb/internal/utils"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

var runCmd = &cobra.Command{
	Use:                "run",
	Short:              "Run the bot with the given configuration.",
	DisableSuggestions: false,
	Run:                runApp,
}

var startTime time.Time = time.Now()

func runApp(cmd *cobra.Command, args []string) {
	utils.InitLogger()
	log := utils.Logger
	mainLogger := log.Named("Main")
	mainLogger.Info("Starting server")
	config.Load(log, cmd)
	router := getRouter(log)

	mainBot, err := bot.StartClient(log)
	if err != nil {
		log.Panic("Failed to start main bot", zap.Error(err))
	}
	cache.InitCache(log)
	workers, err := bot.StartWorkers(log)
	if err != nil {
		log.Panic("Failed to start workers", zap.Error(err))
		return
	}
	workers.AddDefaultClient(mainBot, mainBot.Self)
	bot.StartUserBot(log)
	mainLogger.Info("Server started", zap.Int("port", config.ValueOf.Port))
	mainLogger.Info("File Stream Bot", zap.String("version", versionString))
	mainLogger.Sugar().Infof("Server is running at %s", config.ValueOf.Host)
	publicIp, err := config.GetPublicIP()
	if err != nil {
		mainLogger.Debug("Failed to get public IP", zap.Error(err))
	} else {
		mainLogger.Info("Public IP", zap.String("ip", publicIp))
	}
	//err = router.Run(fmt.Sprintf(":%d", config.ValueOf.Port))
	err = http.ListenAndServeTLS(fmt.Sprintf(":%d", config.ValueOf.Port), "/etc/ssl/cloudflare.crt", "/etc/ssl/cloudflare.key", router)
	if err != nil {
		mainLogger.Sugar().Fatalln(err)
	}
}

func getRouter(log *zap.Logger) *gin.Engine {
	if config.ValueOf.Dev {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.Default()
	router.Use(gin.ErrorLogger())
	router.GET("/", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, types.RootResponse{
			Message: "Server is running.",
			Ok:      true,
			Uptime:  utils.TimeFormat(uint64(time.Since(startTime).Seconds())),
			Version: versionString,
		})
	})
	router.GET("/generate/:message_id", func(ctx *gin.Context) {
		// Print the message to the console
		log.Info("Incoming request")

		// Extract the number from the URL parameters
		message_id_str := ctx.Param("message_id")

		// Call the getStreamLink function with the provided number
		message_id, err := strconv.Atoi(message_id_str)

		if err != nil {
			ctx.String(http.StatusBadRequest, "Invalid number")
			return
		}

		link := getStreamLink(ctx, log, message_id)

		// Return the generated link to the client plaintext not JSON
		ctx.String(http.StatusOK, link)
	})
	routes.Load(log, router)
	return router
}

// A function that receives a number (message_id) and returns the stream link
func getStreamLink(ctx *gin.Context, log *zap.Logger, message_id int) string {
	// Get the file from the message using utils.FileFromMessage(ctx, worker.Client, messageID)
	worker := bot.GetNextWorker()
	file, err := utils.FileFromMessage(ctx, worker.Client, message_id)

	// If there is an error, return the error
	if err != nil {
		log.Info(err.Error())
		return err.Error()
	}

	fullHash := utils.PackFile(
		file.FileName,
		file.FileSize,
		file.MimeType,
		file.ID,
	)

	hash := utils.GetShortHash(fullHash)

	//link := fmt.Sprintf("%s/stream/%d?hash=%s", config.ValueOf.Host, message_id, hash)
	link := fmt.Sprintf("%s/stream/%d.mp4?hash=%s", config.ValueOf.Host, message_id, hash)

	return link
}
