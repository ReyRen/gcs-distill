package server

import (
	"net/http"

	gcsclient "github.com/ReyRen/gcs-distill/internal/client/gcs"
	"github.com/ReyRen/gcs-distill/internal/config"
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
	modelHandler    *handlers.ModelHandler
	resourceHandler *handlers.ResourceHandler
	workflowHandler *handlers.WorkflowHandler
}

// NewRouter 创建路由器
func NewRouter(
	projectSvc service.ProjectService,
	datasetSvc service.DatasetService,
	pipelineSvc service.PipelineService,
	modelSvc service.ModelService,
	gcsClient *gcsclient.Client,
	cfg *config.Config,
	teachers *service.TeacherService,
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
	pipelineHandler := handlers.NewPipelineHandler(pipelineSvc, gcsClient, cfg.Storage)
	modelHandler := handlers.NewModelHandler(modelSvc)
	resourceSvc := service.NewResourceService(gcsClient, cfg.Executor.DataWorkers)
	resourceHandler := handlers.NewResourceHandler(resourceSvc, gcsClient)
	storageConfig := cfg.Storage

	router := &Router{
		engine:          engine,
		projectHandler:  projectHandler,
		datasetHandler:  datasetHandler,
		pipelineHandler: pipelineHandler,
		modelHandler:    modelHandler,
		resourceHandler: resourceHandler,
		workflowHandler: handlers.NewWorkflowHandler(pipelineSvc, projectSvc, datasetSvc, storageConfig),
	}

	router.setupRoutes()
	router.workflowHandler.SetCatalog(teachers, cfg.Executor)
	router.engine.GET("/api/v1/teachers", router.workflowHandler.Teachers)
	router.engine.POST("/api/v1/teachers/test", router.workflowHandler.TestTeacher)
	router.engine.GET("/api/v1/runtime", router.workflowHandler.RuntimeCatalog)

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
	for file, mime := range map[string]string{"swagger-ui.css": "text/css; charset=utf-8", "swagger-ui-bundle.js": "application/javascript; charset=utf-8"} {
		r.engine.GET("/swagger/"+file, func(c *gin.Context) { c.Data(http.StatusOK, mime, apidocs.MustReadFile(file)) })
	}

	// API v1
	v1 := r.engine.Group("/api/v1")
	{
		v1.GET("/workflows", r.workflowHandler.List)
		v1.POST("/workflows/preview", r.workflowHandler.Preview)
		// 项目管理
		projects := v1.Group("/projects")
		{
			projects.POST("", r.projectHandler.CreateProject)
			projects.GET("", r.projectHandler.ListProjects)
			projects.GET("/:id", r.projectHandler.GetProject)
			projects.GET("/:id/summary", r.workflowHandler.Summary)
			projects.PUT("/:id", r.projectHandler.UpdateProject)
			projects.DELETE("/:id", r.projectHandler.DeleteProject)
		}

		// 数据集管理
		datasets := v1.Group("/datasets")
		{
			datasets.POST("", r.datasetHandler.CreateDataset)
			datasets.POST("/upload", r.datasetHandler.UploadDataset)
			datasets.GET("", r.datasetHandler.ListDatasets)
			datasets.GET("/candidates", r.datasetHandler.ListDatasetCandidates)
			datasets.GET("/:id", r.datasetHandler.GetDataset)
			datasets.GET("/:id/preview", r.workflowHandler.PreviewDataset)
			datasets.GET("/:id/download", r.workflowHandler.DownloadDataset)
			datasets.GET("/:id/training-config", r.workflowHandler.DatasetTrainingConfig)
			datasets.PUT("/:id", r.datasetHandler.UpdateDataset)
			datasets.DELETE("/:id", r.datasetHandler.DeleteDataset)
		}

		// 流水线管理
		pipelines := v1.Group("/pipelines")
		pipelines.Use(r.pipelineHandler.AuthorizePipeline)
		{
			pipelines.POST("", r.pipelineHandler.CreatePipeline)
			pipelines.GET("", r.pipelineHandler.ListPipelines)
			pipelines.GET("/:id", r.pipelineHandler.GetPipeline)
			pipelines.POST("/:id/start", r.pipelineHandler.StartPipeline)
			pipelines.POST("/:id/clone", r.pipelineHandler.ClonePipeline)
			pipelines.POST("/:id/cancel", r.pipelineHandler.CancelPipeline)
			pipelines.GET("/:id/stages", r.pipelineHandler.ListStages)
			pipelines.GET("/:id/artifacts", r.workflowHandler.Artifacts)
			pipelines.GET("/:id/artifacts/preview", r.workflowHandler.PreviewArtifact)
			pipelines.GET("/:id/artifacts/download", r.workflowHandler.DownloadArtifact)
			pipelines.GET("/:id/model/download", r.workflowHandler.DownloadModel)
			pipelines.GET("/:id/quality", r.workflowHandler.Quality)
			pipelines.GET("/:id/stages/:stage_id/logs", r.pipelineHandler.GetStageLogs)
			pipelines.GET("/:id/stages/:stage_id/logs/ws", r.pipelineHandler.StreamStageLogsWebSocket)
			pipelines.GET("/:id/stages/:stage_id/logs/download", r.pipelineHandler.DownloadStageLogs)
		}

		// 模型管理
		models := v1.Group("/models")
		{
			models.GET("/student", r.modelHandler.ListStudentModels)
			models.GET("/student/:id", r.modelHandler.GetStudentModel)
		}

		// 资源管理
		resources := v1.Group("/resources")
		{
			resources.GET("/available", r.resourceHandler.Available)
			resources.POST("/validate", r.resourceHandler.Validate)
			resources.GET("/nodes", r.resourceHandler.ListNodes)
			resources.GET("/nodes/:name", r.resourceHandler.GetNode)
		}
	}
}

// Engine 获取 Gin 引擎
func (r *Router) Engine() *gin.Engine {
	return r.engine
}
