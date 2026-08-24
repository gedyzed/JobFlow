package routes

import "github.com/gedyzed/JobFlow/JobService/controllers"

func JobServiceRoutes(router *Router, controller *controllers.JobServiceController) {

	job := router.NewGroup("/jobs")

	job.GET("/:id", controller.GetJobByID)
	job.POST("/", controller.CreateJob)
	job.GET("/", controller.ListJobs)
	job.PUT("/:id", controller.UpdateJob)
	job.DELETE("/:id", controller.DeleteJob)
	job.POST("/:id/cancel", controller.CancelJob)
	job.POST("/:id/retry", controller.RetryJob)

}
