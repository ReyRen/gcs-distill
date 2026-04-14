package server

import (
	"net/http"

	"github.com/ReyRen/gcs-distill/server/apidocs"
	"github.com/ReyRen/gcs-distill/server/handlers"
	"github.com/ReyRen/gcs-distill/server/middleware"
	"github.com/ReyRen/gcs-distill/service"
	"github.com/gin-gonic/gin"
)

// Router 路由器
type Router struct {
	engine *gin.Engine

	// 处理器
	projectHandler  *handlers.ProjectHandler
	datasetHandler  *handlers.DatasetHandler
	pipelineHandler *handlers.PipelineHandler
	resourceHandler *handlers.ResourceHandler
}

// NewRouter 创建路由器
func NewRouter(
	projectSvc service.ProjectService,
	datasetSvc service.DatasetService,
	pipelineSvc service.PipelineService,
	schedulerSvc service.SchedulerService,
) *Router {
	// 设置 Gin 模式
	gin.SetMode(gin.ReleaseMode)

	engine := gin.New()

	// 全局中间件
	engine.Use(middleware.Logger())
	engine.Use(middleware.Recovery())
	engine.Use(middleware.CORS())

	// 创建处理器
	projectHandler := handlers.NewProjectHandler(projectSvc)
	datasetHandler := handlers.NewDatasetHandler(datasetSvc)
	pipelineHandler := handlers.NewPipelineHandler(pipelineSvc)
	resourceHandler := handlers.NewResourceHandler(schedulerSvc)

	router := &Router{
		engine:          engine,
		projectHandler:  projectHandler,
		datasetHandler:  datasetHandler,
		pipelineHandler: pipelineHandler,
		resourceHandler: resourceHandler,
	}

	router.setupRoutes()

	return router
}

// setupRoutes 设置路由
func (r *Router) setupRoutes() {
	// 健康检查
	r.engine.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	r.engine.GET("/swagger", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/swagger/index.html")
	})

	r.engine.GET("/swagger/", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/swagger/index.html")
	})

	r.engine.GET("/swagger/index.html", func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html; charset=utf-8", apidocs.MustReadFile("index.html"))
	})
	r.engine.GET("/swagger/openapi.json", func(c *gin.Context) {
		c.Data(http.StatusOK, "application/json; charset=utf-8", apidocs.MustReadFile("openapi.json"))
	})

	// API v1
	v1 := r.engine.Group("/api/v1")
	{
		// 项目管理
		projects := v1.Group("/projects")
		{
			projects.POST("", r.projectHandler.CreateProject)
			projects.GET("", r.projectHandler.ListProjects)
			projects.GET("/:id", r.projectHandler.GetProject)
			projects.POST("/:id/datasets", r.datasetHandler.CreateDataset)
			projects.PUT("/:id", r.projectHandler.UpdateProject)
			projects.DELETE("/:id", r.projectHandler.DeleteProject)
		}

		// 数据集管理
		datasets := v1.Group("/datasets")
		{
			datasets.POST("", r.datasetHandler.CreateDataset)
			datasets.GET("", r.datasetHandler.ListDatasets)
			datasets.GET("/:id", r.datasetHandler.GetDataset)
			datasets.PUT("/:id", r.datasetHandler.UpdateDataset)
			datasets.DELETE("/:id", r.datasetHandler.DeleteDataset)
		}

		// 流水线管理
		pipelines := v1.Group("/pipelines")
		{
			pipelines.POST("", r.pipelineHandler.CreatePipeline)
			pipelines.GET("", r.pipelineHandler.ListPipelines)
			pipelines.GET("/:id", r.pipelineHandler.GetPipeline)
			pipelines.POST("/:id/start", r.pipelineHandler.StartPipeline)
			pipelines.POST("/:id/cancel", r.pipelineHandler.CancelPipeline)
			pipelines.GET("/:id/stages", r.pipelineHandler.ListStages)
			pipelines.GET("/:id/stages/:stage_id/logs", r.pipelineHandler.GetStageLogs)
			pipelines.GET("/:id/stages/:stage_id/logs/stream", r.pipelineHandler.StreamStageLogs)
			pipelines.GET("/:id/stages/:stage_id/logs/download", r.pipelineHandler.DownloadStageLogs)
		}

		// 资源管理
		resources := v1.Group("/resources")
		{
			resources.GET("/nodes", r.resourceHandler.ListNodes)
			resources.GET("/nodes/:name", r.resourceHandler.GetNode)
		}
	}
}

// Engine 获取 Gin 引擎
func (r *Router) Engine() *gin.Engine {
	return r.engine
}
