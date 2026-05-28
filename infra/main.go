package main

import (
	neonsdk "github.com/kislerdm/pulumi-neon/sdk/go/neon"
	flysdk "github.com/pulumiverse/pulumi-fly/sdk/go/fly"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

// main provisions:
//   - A Neon serverless Postgres project (replaces docker-compose postgres)
//   - A Fly.io app with secrets wired from Pulumi config
//
// Usage:
//
//	cd infra
//	pulumi config set --secret jwtSecret <secret>
//	pulumi up
func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cfg := config.New(ctx, "")

		// --- Neon (serverless Postgres) ---
		// Neon connection strings include sslmode=require by default.
		project, err := neonsdk.NewProject(ctx, "ardoise", &neonsdk.ProjectArgs{
			Name:      pulumi.StringInput(pulumi.String("ardoise")),
			RegionId:  pulumi.StringInput(pulumi.String("aws-us-east-1")),
			PgVersion: pulumi.IntInput(pulumi.Int(15)),
		})
		if err != nil {
			return err
		}

		// --- Fly.io app ---
		app, err := flysdk.NewApp(ctx, "ardoise-api", &flysdk.AppArgs{
			Name: pulumi.String("ardoise-api"),
		})
		if err != nil {
			return err
		}

		jwtSecret := cfg.RequireSecret("jwtSecret")

		_, err = flysdk.NewSecret(ctx, "secret-db-url", &flysdk.SecretArgs{
			App:   app.Name,
			Name:  pulumi.String("DATABASE_URL"),
			Value: project.ConnectionUri.ApplyT(func(v string) string { return v }).(pulumi.StringOutput),
		})
		if err != nil {
			return err
		}

		_, err = flysdk.NewSecret(ctx, "secret-jwt-secret", &flysdk.SecretArgs{
			App:   app.Name,
			Name:  pulumi.String("JWT_SECRET"),
			Value: jwtSecret,
		})
		if err != nil {
			return err
		}

		ctx.Export("apiUrl", pulumi.Sprintf("https://%s.fly.dev", app.Name))
		ctx.Export("dbUrl", project.ConnectionUri)

		return nil
	})
}
