package initializer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectSpringBootGradle(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "build.gradle"), []byte(`
plugins {
    id 'org.springframework.boot' version '3.2.0'
}
`), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	primary := det.Primary()
	if primary.Name != "springboot" {
		t.Errorf("expected springboot, got %q", primary.Name)
	}
	if primary.BuildTool != "gradle" {
		t.Errorf("expected gradle, got %q", primary.BuildTool)
	}
}

func TestDetectSpringBootMaven(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "pom.xml"), []byte(`
<project>
    <parent>
        <artifactId>spring-boot-starter-parent</artifactId>
    </parent>
</project>
`), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	primary := det.Primary()
	if primary.Name != "springboot" {
		t.Errorf("expected springboot, got %q", primary.Name)
	}
	if primary.BuildTool != "maven" {
		t.Errorf("expected maven, got %q", primary.BuildTool)
	}
}

func TestDetectNode(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name": "test"}`), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	primary := det.Primary()
	if primary.Name != "node" {
		t.Errorf("expected node, got %q", primary.Name)
	}
	if primary.BuildTool != "npm" {
		t.Errorf("expected npm, got %q", primary.BuildTool)
	}
}

func TestDetectNodeYarn(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name": "test"}`), 0644)
	os.WriteFile(filepath.Join(dir, "yarn.lock"), []byte(""), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if det.Primary().BuildTool != "yarn" {
		t.Errorf("expected yarn, got %q", det.Primary().BuildTool)
	}
}

func TestDetectPython(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "requirements.txt"), []byte("flask==3.0.0"), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	primary := det.Primary()
	if primary.Name != "python" {
		t.Errorf("expected python, got %q", primary.Name)
	}
}

func TestDetectGo(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test"), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if det.Primary().Name != "go" {
		t.Errorf("expected go, got %q", det.Primary().Name)
	}
}

func TestDetectRust(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte("[package]"), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if det.Primary().Name != "rust" {
		t.Errorf("expected rust, got %q", det.Primary().Name)
	}
}

func TestDetectGeneric(t *testing.T) {
	dir := t.TempDir()

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if det.Primary().Name != "generic" {
		t.Errorf("expected generic, got %q", det.Primary().Name)
	}
}

func TestDetectMultiModuleGradle(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "build.gradle"), []byte(`
plugins { id 'org.springframework.boot' version '3.2.0' }
`), 0644)
	os.WriteFile(filepath.Join(dir, "settings.gradle"), []byte(`
rootProject.name = 'my-platform'
include 'api-server'
include 'batch'
include 'common'
`), 0644)
	os.MkdirAll(filepath.Join(dir, "api-server"), 0755)
	os.MkdirAll(filepath.Join(dir, "batch"), 0755)
	os.MkdirAll(filepath.Join(dir, "common"), 0755)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !det.MultiModule {
		t.Error("expected multi-module detection")
	}
	if len(det.ModuleNames) != 3 {
		t.Errorf("expected 3 modules, got %d", len(det.ModuleNames))
	}

	// common should be filtered out from deployable
	if len(det.DeployableModules) != 2 {
		t.Errorf("expected 2 deployable modules, got %d: %v", len(det.DeployableModules), det.DeployableModules)
	}
}

func TestFilterDeployableModules(t *testing.T) {
	dir := t.TempDir()
	modules := []string{
		"api-server", "admin", "common", "api-common",
		"spring-support", "protobuf", "kafka-common",
	}

	result := filterDeployableModules(dir, modules)

	// Should keep: api-server, admin
	// Should filter: common, api-common, spring-support, protobuf, kafka-common
	if len(result) != 2 {
		t.Errorf("expected 2 deployable modules, got %d: %v", len(result), result)
	}
}

func TestGenerateConfig(t *testing.T) {
	det := &DetectedProject{
		ProjectName:       "test-app",
		RootDir:           "/tmp/test",
		DeployableModules: []string{"test-app"},
		Types: []ProjectType{
			{Name: "springboot", BuildTool: "gradle", Confidence: 95, Details: "Spring Boot with Gradle"},
		},
	}

	config := GenerateConfig(det)
	if config == "" {
		t.Fatal("expected non-empty config")
	}

	// Check key fields exist
	checks := []string{"project:", "name: test-app", "environments:", "modules:", "type: springboot"}
	for _, check := range checks {
		if !searchString(config, check) {
			t.Errorf("expected config to contain %q", check)
		}
	}
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// --- PHP Detection ---

func TestDetectPHPComposer(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "composer.json"), []byte(`{"require": {"php": ">=8.0"}}`), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if det.Primary().Name != "php" {
		t.Errorf("expected php, got %q", det.Primary().Name)
	}
}

func TestDetectPHPLaravel(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "composer.json"), []byte(`{"require": {"laravel/framework": "^10.0"}}`), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	primary := det.Primary()
	if primary.Name != "php" {
		t.Errorf("expected php, got %q", primary.Name)
	}
	if !searchString(primary.Details, "Laravel") {
		t.Errorf("expected Laravel details, got %q", primary.Details)
	}
	if primary.Confidence < 90 {
		t.Errorf("expected high confidence for Laravel, got %d", primary.Confidence)
	}
}

func TestDetectPHPSymfony(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "composer.json"), []byte(`{"require": {"symfony/framework-bundle": "^6.0"}}`), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !searchString(det.Primary().Details, "Symfony") {
		t.Errorf("expected Symfony details, got %q", det.Primary().Details)
	}
}

func TestDetectPHPCodeIgniter(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "composer.json"), []byte(`{"require": {"codeigniter4/framework": "^4.0"}}`), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !searchString(det.Primary().Details, "CodeIgniter") {
		t.Errorf("expected CodeIgniter details, got %q", det.Primary().Details)
	}
}

func TestDetectPHPSlim(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "composer.json"), []byte(`{"require": {"slim/slim": "^4.0"}}`), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !searchString(det.Primary().Details, "Slim") {
		t.Errorf("expected Slim details, got %q", det.Primary().Details)
	}
}

func TestDetectPHPCakePHP(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "composer.json"), []byte(`{"require": {"cakephp/cakephp": "^5.0"}}`), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !searchString(det.Primary().Details, "CakePHP") {
		t.Errorf("expected CakePHP details, got %q", det.Primary().Details)
	}
}

// --- Ruby Detection ---

func TestDetectRuby(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "Gemfile"), []byte("source 'https://rubygems.org'\ngem 'rack'"), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if det.Primary().Name != "ruby" {
		t.Errorf("expected ruby, got %q", det.Primary().Name)
	}
}

func TestDetectRubyRails(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "Gemfile"), []byte("gem 'rails', '~> 7.0'"), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !searchString(det.Primary().Details, "Ruby on Rails") {
		t.Errorf("expected Rails details, got %q", det.Primary().Details)
	}
	if det.Primary().Confidence < 90 {
		t.Errorf("expected high confidence for Rails, got %d", det.Primary().Confidence)
	}
}

func TestDetectRubySinatra(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "Gemfile"), []byte("gem 'sinatra'"), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !searchString(det.Primary().Details, "Sinatra") {
		t.Errorf("expected Sinatra details, got %q", det.Primary().Details)
	}
}

func TestDetectRubyHanami(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "Gemfile"), []byte("gem 'hanami', '~> 2.0'"), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !searchString(det.Primary().Details, "Hanami") {
		t.Errorf("expected Hanami details, got %q", det.Primary().Details)
	}
}

func TestDetectRubyGrape(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "Gemfile"), []byte("gem 'grape'"), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !searchString(det.Primary().Details, "Grape") {
		t.Errorf("expected Grape details, got %q", det.Primary().Details)
	}
}

func TestDetectRubyRoda(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "Gemfile"), []byte("gem 'roda'"), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !searchString(det.Primary().Details, "Roda") {
		t.Errorf("expected Roda details, got %q", det.Primary().Details)
	}
}

// --- .NET Detection ---

func TestDetectDotNet(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "MyApp.csproj"), []byte(`
<Project Sdk="Microsoft.NET.Sdk.Web">
  <ItemGroup>
    <PackageReference Include="Microsoft.AspNetCore" />
  </ItemGroup>
</Project>
`), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if det.Primary().Name != "dotnet" {
		t.Errorf("expected dotnet, got %q", det.Primary().Name)
	}
}

func TestDetectDotNetBlazor(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "MyApp.csproj"), []byte(`
<Project>
  <ItemGroup>
    <PackageReference Include="Microsoft.AspNetCore.Components.Server" />
  </ItemGroup>
</Project>
`), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !searchString(det.Primary().Details, "Blazor") {
		t.Errorf("expected Blazor details, got %q", det.Primary().Details)
	}
}

func TestDetectDotNetInSubdir(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "src")
	os.MkdirAll(subDir, 0755)
	os.WriteFile(filepath.Join(subDir, "MyApp.csproj"), []byte(`<Project Sdk="Microsoft.NET.Sdk.Web"></Project>`), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if det.Primary().Name != "dotnet" {
		t.Errorf("expected dotnet, got %q", det.Primary().Name)
	}
}

func TestDetectDotNetWithSolution(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "MyApp.sln"), []byte("solution file"), 0644)
	os.WriteFile(filepath.Join(dir, "MyApp.csproj"), []byte(`<Project></Project>`), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !searchString(det.Primary().Details, "Solution") {
		t.Errorf("expected Solution in details, got %q", det.Primary().Details)
	}
}

// --- Kotlin Detection ---

func TestDetectKotlin(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "build.gradle.kts"), []byte(`
plugins {
    kotlin("jvm") version "1.9.0"
}
`), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if det.Primary().Name != "kotlin" {
		t.Errorf("expected kotlin, got %q", det.Primary().Name)
	}
}

func TestDetectKotlinKtor(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "build.gradle.kts"), []byte(`
plugins {
    kotlin("jvm") version "1.9.0"
}
dependencies {
    implementation("io.ktor:ktor-server-core:2.3.0")
}
`), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !searchString(det.Primary().Details, "Ktor") {
		t.Errorf("expected Ktor details, got %q", det.Primary().Details)
	}
}

func TestDetectKotlinMicronaut(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "build.gradle.kts"), []byte(`
plugins {
    kotlin("jvm") version "1.9.0"
    id("io.micronaut.application")
}
`), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !searchString(det.Primary().Details, "Micronaut") {
		t.Errorf("expected Micronaut details, got %q", det.Primary().Details)
	}
}

func TestDetectKotlinJavalin(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "build.gradle.kts"), []byte(`
plugins {
    kotlin("jvm")
}
dependencies {
    implementation("io.javalin:javalin:5.6.0")
}
`), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !searchString(det.Primary().Details, "Javalin") {
		t.Errorf("expected Javalin details, got %q", det.Primary().Details)
	}
}

func TestDetectKotlinHttp4k(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "build.gradle.kts"), []byte(`
plugins {
    kotlin("jvm")
}
dependencies {
    implementation("org.http4k:http4k-core:5.0.0")
}
`), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !searchString(det.Primary().Details, "http4k") {
		t.Errorf("expected http4k details, got %q", det.Primary().Details)
	}
}

func TestDetectKotlinSpringBootSkipped(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "build.gradle.kts"), []byte(`
plugins {
    kotlin("jvm")
    id("org.springframework.boot") version "3.2.0"
}
`), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should be springboot, not kotlin
	if det.Primary().Name != "springboot" {
		t.Errorf("expected springboot for Kotlin+Spring Boot, got %q", det.Primary().Name)
	}
}

// --- Elixir Detection ---

func TestDetectElixir(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "mix.exs"), []byte(`
defmodule MyApp.MixProject do
  use Mix.Project
end
`), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if det.Primary().Name != "elixir" {
		t.Errorf("expected elixir, got %q", det.Primary().Name)
	}
}

func TestDetectElixirPhoenix(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "mix.exs"), []byte(`
defp deps do
  [{:phoenix, "~> 1.7"}]
end
`), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !searchString(det.Primary().Details, "Phoenix") {
		t.Errorf("expected Phoenix details, got %q", det.Primary().Details)
	}
	if det.Primary().Confidence < 90 {
		t.Errorf("expected high confidence for Phoenix, got %d", det.Primary().Confidence)
	}
}

func TestDetectElixirAbsinthe(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "mix.exs"), []byte(`
defp deps do
  [{:absinthe, "~> 1.7"}]
end
`), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !searchString(det.Primary().Details, "Absinthe") {
		t.Errorf("expected Absinthe details, got %q", det.Primary().Details)
	}
}

func TestDetectElixirBandit(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "mix.exs"), []byte(`
defp deps do
  [{:bandit, "~> 1.0"}]
end
`), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !searchString(det.Primary().Details, "Bandit") {
		t.Errorf("expected Bandit details, got %q", det.Primary().Details)
	}
}

func TestDetectElixirPlug(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "mix.exs"), []byte(`
defp deps do
  [{:plug_cowboy, "~> 2.0"}]
end
`), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !searchString(det.Primary().Details, "Plug") {
		t.Errorf("expected Plug details, got %q", det.Primary().Details)
	}
}

// --- Go Framework Detection ---

func TestDetectGoGin(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte(`
module myapp

go 1.21

require github.com/gin-gonic/gin v1.9.0
`), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !searchString(det.Primary().Details, "Gin") {
		t.Errorf("expected Gin details, got %q", det.Primary().Details)
	}
}

func TestDetectGoEcho(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte(`
module myapp

require github.com/labstack/echo/v4 v4.11.0
`), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !searchString(det.Primary().Details, "Echo") {
		t.Errorf("expected Echo details, got %q", det.Primary().Details)
	}
}

func TestDetectGoFiber(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte(`
module myapp

require github.com/gofiber/fiber/v2 v2.50.0
`), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !searchString(det.Primary().Details, "Fiber") {
		t.Errorf("expected Fiber details, got %q", det.Primary().Details)
	}
}

func TestDetectGoChi(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte(`
module myapp

require github.com/go-chi/chi/v5 v5.0.10
`), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !searchString(det.Primary().Details, "Chi") {
		t.Errorf("expected Chi details, got %q", det.Primary().Details)
	}
}

// --- Rust Framework Detection ---

func TestDetectRustAxum(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte(`
[package]
name = "myapp"

[dependencies]
axum = "0.7"
`), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !searchString(det.Primary().Details, "Axum") {
		t.Errorf("expected Axum details, got %q", det.Primary().Details)
	}
}

func TestDetectRustActix(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte(`
[dependencies]
actix-web = "4"
`), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !searchString(det.Primary().Details, "Actix") {
		t.Errorf("expected Actix details, got %q", det.Primary().Details)
	}
}

func TestDetectRustRocket(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte(`
[dependencies]
rocket = "0.5"
`), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !searchString(det.Primary().Details, "Rocket") {
		t.Errorf("expected Rocket details, got %q", det.Primary().Details)
	}
}

func TestDetectRustWarp(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte(`
[dependencies]
warp = "0.3"
`), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !searchString(det.Primary().Details, "Warp") {
		t.Errorf("expected Warp details, got %q", det.Primary().Details)
	}
}

func TestDetectRustPoem(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte(`
[dependencies]
poem = "2.0"
`), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !searchString(det.Primary().Details, "Poem") {
		t.Errorf("expected Poem details, got %q", det.Primary().Details)
	}
}

// --- Node Framework Detection ---

func TestDetectNodePnpm(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name": "test"}`), 0644)
	os.WriteFile(filepath.Join(dir, "pnpm-lock.yaml"), []byte(""), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if det.Primary().BuildTool != "pnpm" {
		t.Errorf("expected pnpm, got %q", det.Primary().BuildTool)
	}
}

func TestDetectNodeNextJS(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"dependencies": {"next": "^14.0.0"}}`), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !searchString(det.Primary().Details, "Next.js") {
		t.Errorf("expected Next.js details, got %q", det.Primary().Details)
	}
}

func TestDetectNodeNestJS(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"dependencies": {"@nestjs/core": "^10.0.0"}}`), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !searchString(det.Primary().Details, "NestJS") {
		t.Errorf("expected NestJS details, got %q", det.Primary().Details)
	}
}

func TestDetectNodeExpress(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"dependencies": {"express": "^4.18.0"}}`), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !searchString(det.Primary().Details, "Express") {
		t.Errorf("expected Express details, got %q", det.Primary().Details)
	}
}

func TestDetectNodeFastify(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"dependencies": {"fastify": "^4.0.0"}}`), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !searchString(det.Primary().Details, "Fastify") {
		t.Errorf("expected Fastify details, got %q", det.Primary().Details)
	}
}

func TestDetectNodeHono(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"dependencies": {"hono": "^3.0.0"}}`), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !searchString(det.Primary().Details, "Hono") {
		t.Errorf("expected Hono details, got %q", det.Primary().Details)
	}
}

func TestDetectNodeKoa(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"dependencies": {"koa": "^2.0.0"}}`), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !searchString(det.Primary().Details, "Koa") {
		t.Errorf("expected Koa details, got %q", det.Primary().Details)
	}
}

func TestDetectNodeNuxt(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"dependencies": {"nuxt": "^3.0.0"}}`), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !searchString(det.Primary().Details, "Nuxt") {
		t.Errorf("expected Nuxt details, got %q", det.Primary().Details)
	}
}

func TestDetectNodeMonorepo(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name": "test"}`), 0644)
	os.WriteFile(filepath.Join(dir, "turbo.json"), []byte("{}"), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !searchString(det.Primary().Details, "monorepo") {
		t.Errorf("expected monorepo in details, got %q", det.Primary().Details)
	}
}

// --- Python Detection ---

func TestDetectPythonPoetry(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte("[tool.poetry]\nname = \"myapp\""), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	primary := det.Primary()
	if primary.Name != "python" {
		t.Errorf("expected python, got %q", primary.Name)
	}
	if primary.BuildTool != "poetry" {
		t.Errorf("expected poetry, got %q", primary.BuildTool)
	}
}

func TestDetectPythonUV(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte("[tool.uv]\nname = \"myapp\""), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if det.Primary().BuildTool != "uv" {
		t.Errorf("expected uv, got %q", det.Primary().BuildTool)
	}
}

func TestDetectPythonUVLock(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte("[project]\nname = \"myapp\""), 0644)
	os.WriteFile(filepath.Join(dir, "uv.lock"), []byte(""), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if det.Primary().BuildTool != "uv" {
		t.Errorf("expected uv, got %q", det.Primary().BuildTool)
	}
}

func TestDetectPythonFastAPI(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "requirements.txt"), []byte("fastapi==0.104.0\nuvicorn"), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !searchString(det.Primary().Details, "FastAPI") {
		t.Errorf("expected FastAPI details, got %q", det.Primary().Details)
	}
}

func TestDetectPythonDjango(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "requirements.txt"), []byte("Django==5.0\ngunicorn"), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !searchString(det.Primary().Details, "Django") {
		t.Errorf("expected Django details, got %q", det.Primary().Details)
	}
}

func TestDetectPythonFlask(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "requirements.txt"), []byte("flask==3.0.0"), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !searchString(det.Primary().Details, "Flask") {
		t.Errorf("expected Flask details, got %q", det.Primary().Details)
	}
}

func TestDetectPythonSetupPy(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "setup.py"), []byte("from setuptools import setup\nsetup()"), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	primary := det.Primary()
	if primary.Name != "python" {
		t.Errorf("expected python, got %q", primary.Name)
	}
	if primary.BuildTool != "pip" {
		t.Errorf("expected pip, got %q", primary.BuildTool)
	}
}

func TestDetectPythonPyprojectFastAPI(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte(`
[tool.poetry]
name = "myapi"

[tool.poetry.dependencies]
fastapi = "^0.104"
`), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !searchString(det.Primary().Details, "FastAPI") {
		t.Errorf("expected FastAPI details, got %q", det.Primary().Details)
	}
}

func TestDetectPythonPyprojectDjango(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte(`
[tool.poetry]
name = "myapp"

[tool.poetry.dependencies]
django = "^5.0"
`), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !searchString(det.Primary().Details, "Django") {
		t.Errorf("expected Django details, got %q", det.Primary().Details)
	}
}

func TestDetectPythonPyprojectFlask(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte(`
[tool.poetry]
name = "myapp"

[tool.poetry.dependencies]
flask = "^3.0"
`), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !searchString(det.Primary().Details, "Flask") {
		t.Errorf("expected Flask details, got %q", det.Primary().Details)
	}
}

func TestDetectPythonPyprojectLitestar(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte(`
[tool.poetry]
name = "myapp"

[tool.poetry.dependencies]
litestar = "^2.0"
`), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !searchString(det.Primary().Details, "Litestar") {
		t.Errorf("expected Litestar details, got %q", det.Primary().Details)
	}
}

func TestDetectPythonPyprojectTornado(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte(`
[tool.poetry]
name = "myapp"

[tool.poetry.dependencies]
tornado = "^6.0"
`), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !searchString(det.Primary().Details, "Tornado") {
		t.Errorf("expected Tornado details, got %q", det.Primary().Details)
	}
}

// --- Java Framework Detection ---

func TestDetectJavaQuarkusGradle(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "build.gradle"), []byte(`
plugins {
    id 'io.quarkus' version '3.6.0'
}
`), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !searchString(det.Primary().Details, "Quarkus") {
		t.Errorf("expected Quarkus details, got %q", det.Primary().Details)
	}
}

func TestDetectJavaMicronautGradle(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "build.gradle"), []byte(`
plugins {
    id 'io.micronaut.application' version '4.0.0'
}
`), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !searchString(det.Primary().Details, "Micronaut") {
		t.Errorf("expected Micronaut details, got %q", det.Primary().Details)
	}
}

func TestDetectJavaDropwizardGradle(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "build.gradle"), []byte(`
dependencies {
    implementation 'io.dropwizard:dropwizard-core:4.0.0'
}
`), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !searchString(det.Primary().Details, "Dropwizard") {
		t.Errorf("expected Dropwizard details, got %q", det.Primary().Details)
	}
}

func TestDetectJavaQuarkusMaven(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "pom.xml"), []byte(`
<project>
    <dependencies>
        <dependency>
            <groupId>io.quarkus</groupId>
        </dependency>
    </dependencies>
</project>
`), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !searchString(det.Primary().Details, "Quarkus") {
		t.Errorf("expected Quarkus details, got %q", det.Primary().Details)
	}
}

// --- Spring Boot build.gradle.kts ---

func TestDetectSpringBootGradleKts(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "build.gradle.kts"), []byte(`
plugins {
    id("org.springframework.boot") version "3.2.0"
}
`), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	primary := det.Primary()
	if primary.Name != "springboot" {
		t.Errorf("expected springboot, got %q", primary.Name)
	}
	if primary.BuildTool != "gradle" {
		t.Errorf("expected gradle, got %q", primary.BuildTool)
	}
}

// --- Multi-Module Detection ---

func TestDetectMultiModuleMaven(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "pom.xml"), []byte(`
<project>
    <groupId>com.example</groupId>
    <modules>
        <module>api</module>
        <module>worker</module>
        <module>common</module>
    </modules>
    <dependencies>
        <dependency>
            <groupId>org.springframework.boot</groupId>
        </dependency>
    </dependencies>
</project>
`), 0644)
	os.MkdirAll(filepath.Join(dir, "api"), 0755)
	os.MkdirAll(filepath.Join(dir, "worker"), 0755)
	os.MkdirAll(filepath.Join(dir, "common"), 0755)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !det.MultiModule {
		t.Error("expected multi-module detection")
	}
	if len(det.ModuleNames) != 3 {
		t.Errorf("expected 3 modules, got %d", len(det.ModuleNames))
	}
}

func TestDetectMultiModuleGradleKts(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "build.gradle.kts"), []byte(`
plugins {
    id("org.springframework.boot") version "3.2.0"
}
`), 0644)
	os.WriteFile(filepath.Join(dir, "settings.gradle.kts"), []byte(`
rootProject.name = "my-project"
include("api-server")
include("batch")
include("common")
`), 0644)
	os.MkdirAll(filepath.Join(dir, "api-server"), 0755)
	os.MkdirAll(filepath.Join(dir, "batch"), 0755)
	os.MkdirAll(filepath.Join(dir, "common"), 0755)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !det.MultiModule {
		t.Error("expected multi-module detection for settings.gradle.kts")
	}
}

// --- Helper Functions ---

func TestParseGradleIncludes(t *testing.T) {
	tests := []struct {
		name    string
		content string
		count   int
	}{
		{"single quote", "include 'api-server'", 1},
		{"double quote", `include "api-server"`, 1},
		{"multiple", "include 'api', 'worker', 'common'", 3},
		{"kts style", `include("api-server")`, 1},
		{"mixed", `include 'api'
include 'worker'`, 2},
		{"empty", "", 0},
		{"no includes", "rootProject.name = 'test'", 0},
		{"with colon prefix", "include ':api-server'", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseGradleIncludes(tt.content)
			if len(result) != tt.count {
				t.Errorf("expected %d modules, got %d: %v", tt.count, len(result), result)
			}
		})
	}
}

func TestParseMavenModules(t *testing.T) {
	tests := []struct {
		name    string
		content string
		count   int
	}{
		{"basic", `<modules>
    <module>api</module>
    <module>worker</module>
</modules>`, 2},
		{"single", `<modules>
    <module>api</module>
</modules>`, 1},
		{"empty modules", `<modules>
</modules>`, 0},
		{"no modules tag", `<project></project>`, 0},
		{"empty string", "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseMavenModules(tt.content)
			if len(result) != tt.count {
				t.Errorf("expected %d modules, got %d: %v", tt.count, len(result), result)
			}
		})
	}
}

func TestCapitalize(t *testing.T) {
	tests := []struct {
		input, expected string
	}{
		{"gradle", "Gradle"},
		{"maven", "Maven"},
		{"npm", "Npm"},
		{"", ""},
		{"A", "A"},
	}

	for _, tt := range tests {
		result := capitalize(tt.input)
		if result != tt.expected {
			t.Errorf("capitalize(%q) = %q, expected %q", tt.input, result, tt.expected)
		}
	}
}

func TestFilterDeployableModulesEdgeCases(t *testing.T) {
	dir := t.TempDir()

	tests := []struct {
		name     string
		modules  []string
		expected int
	}{
		{"all deployable", []string{"api-server", "admin", "batch"}, 3},
		{"with common", []string{"api-server", "common"}, 1},
		{"with lib suffix", []string{"api-server", "my-lib"}, 1},
		{"with shared suffix", []string{"api", "data-shared"}, 1},
		{"with core suffix", []string{"api", "app-core"}, 1},
		{"with support suffix", []string{"api", "spring-support"}, 1},
		{"with sdk suffix", []string{"api", "java-sdk"}, 1},
		{"with utils suffix", []string{"api", "common-utils"}, 1},
		{"exact match common", []string{"common"}, 1}, // returns all if all filtered
		{"exact match model", []string{"model"}, 1},   // returns all if all filtered
		{"with proto", []string{"api", "proto"}, 1},
		{"all library returns all", []string{"common", "shared", "utils"}, 3}, // returns all if all filtered
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterDeployableModules(dir, tt.modules)
			if len(result) != tt.expected {
				t.Errorf("expected %d deployable, got %d: %v", tt.expected, len(result), result)
			}
		})
	}
}

func TestFilterDeployableModulesWithBuildGradle(t *testing.T) {
	dir := t.TempDir()

	// Module with bootJar disabled
	modDir := filepath.Join(dir, "common-lib")
	os.MkdirAll(modDir, 0755)
	os.WriteFile(filepath.Join(modDir, "build.gradle"), []byte(`
bootJar {
    enabled = false
}
`), 0644)

	// Module with bootJar enabled
	apiDir := filepath.Join(dir, "api")
	os.MkdirAll(apiDir, 0755)
	os.WriteFile(filepath.Join(apiDir, "build.gradle"), []byte(`
bootJar {
    mainClass = 'com.example.Application'
}
`), 0644)

	result := filterDeployableModules(dir, []string{"api", "common-lib"})
	if len(result) != 1 || result[0] != "api" {
		t.Errorf("expected only api, got %v", result)
	}
}

// --- Primary() edge cases ---

func TestPrimaryEmpty(t *testing.T) {
	det := &DetectedProject{Types: []ProjectType{}}
	primary := det.Primary()
	if primary.Name != "generic" {
		t.Errorf("expected generic for empty types, got %q", primary.Name)
	}
}

func TestPrimaryHighestConfidence(t *testing.T) {
	det := &DetectedProject{
		Types: []ProjectType{
			{Name: "java", Confidence: 85},
			{Name: "springboot", Confidence: 95},
			{Name: "node", Confidence: 50},
		},
	}
	primary := det.Primary()
	if primary.Name != "springboot" {
		t.Errorf("expected springboot (highest confidence), got %q", primary.Name)
	}
}

// --- Additional Detection Features ---

func TestDetectDocker(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte("FROM node:18"), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !det.HasDocker {
		t.Error("expected Docker detection")
	}
}

func TestDetectDockerCompose(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "docker-compose.yml"), []byte("version: '3'"), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !det.HasDocker {
		t.Error("expected Docker detection via docker-compose")
	}
}

func TestDetectCI(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".github", "workflows"), 0755)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !det.HasCI {
		t.Error("expected CI detection via GitHub Actions")
	}
}

func TestDetectCIGitLab(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".gitlab-ci.yml"), []byte("stages: [build]"), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !det.HasCI {
		t.Error("expected CI detection via GitLab CI")
	}
}

func TestDetectCIJenkins(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "Jenkinsfile"), []byte("pipeline {}"), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !det.HasCI {
		t.Error("expected CI detection via Jenkinsfile")
	}
}

func TestDetectScriptDir(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "script"), 0755)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !det.HasScriptDir {
		t.Error("expected script dir detection")
	}
}

func TestDetectConfigDir(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "config"), 0755)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !det.HasConfigDir {
		t.Error("expected config dir detection")
	}
}

func TestDetectExternalPkg(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "external-package"), 0755)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !det.HasExternalPkg {
		t.Error("expected external-package detection")
	}
}

// --- GenerateConfig Tests ---

func TestGenerateConfigNode(t *testing.T) {
	det := &DetectedProject{
		ProjectName:       "frontend",
		RootDir:           "/tmp/test",
		DeployableModules: []string{"frontend"},
		Types: []ProjectType{
			{Name: "node", BuildTool: "npm", Confidence: 85, Details: "Node.js with npm"},
		},
	}

	config := GenerateConfig(det)
	checks := []string{"type: node", "port: 3000", "npm"}
	for _, check := range checks {
		if !searchString(config, check) {
			t.Errorf("expected config to contain %q", check)
		}
	}
}

func TestGenerateConfigPython(t *testing.T) {
	det := &DetectedProject{
		ProjectName:       "api",
		RootDir:           "/tmp/test",
		DeployableModules: []string{"api"},
		Types: []ProjectType{
			{Name: "python", BuildTool: "poetry", Confidence: 90, Details: "Python with Poetry"},
		},
	}

	config := GenerateConfig(det)
	checks := []string{"type: python", "port: 8000", "poetry"}
	for _, check := range checks {
		if !searchString(config, check) {
			t.Errorf("expected config to contain %q", check)
		}
	}
}

func TestGenerateConfigGo(t *testing.T) {
	det := &DetectedProject{
		ProjectName:       "myapp",
		RootDir:           "/tmp/test",
		DeployableModules: []string{"myapp"},
		Types: []ProjectType{
			{Name: "go", BuildTool: "go", Confidence: 90, Details: "Go"},
		},
	}

	config := GenerateConfig(det)
	checks := []string{"type: go", "go build"}
	for _, check := range checks {
		if !searchString(config, check) {
			t.Errorf("expected config to contain %q", check)
		}
	}
}

func TestGenerateConfigRust(t *testing.T) {
	det := &DetectedProject{
		ProjectName:       "myapp",
		RootDir:           "/tmp/test",
		DeployableModules: []string{"myapp"},
		Types: []ProjectType{
			{Name: "rust", BuildTool: "cargo", Confidence: 90, Details: "Rust with Cargo"},
		},
	}

	config := GenerateConfig(det)
	checks := []string{"type: rust", "cargo build"}
	for _, check := range checks {
		if !searchString(config, check) {
			t.Errorf("expected config to contain %q", check)
		}
	}
}

func TestGenerateConfigGeneric(t *testing.T) {
	det := &DetectedProject{
		ProjectName:       "unknown",
		RootDir:           "/tmp/test",
		DeployableModules: []string{"unknown"},
		Types: []ProjectType{
			{Name: "generic", Confidence: 30, Details: "No specific project type detected"},
		},
	}

	config := GenerateConfig(det)
	if !searchString(config, "type: generic") {
		t.Error("expected generic type in config")
	}
}

func TestGenerateConfigMultiModule(t *testing.T) {
	det := &DetectedProject{
		ProjectName:       "my-platform",
		RootDir:           "/tmp/test",
		MultiModule:       true,
		ModuleNames:       []string{"api-server", "batch", "common"},
		DeployableModules: []string{"api-server", "batch"},
		Types: []ProjectType{
			{Name: "springboot", BuildTool: "gradle", Confidence: 95, Details: "Spring Boot with Gradle"},
		},
	}

	config := GenerateConfig(det)

	if !searchString(config, "api-server:") {
		t.Error("expected api-server module in config")
	}
	if !searchString(config, "batch:") {
		t.Error("expected batch module in config")
	}
	if !searchString(config, "Library modules excluded") {
		t.Error("expected library modules comment")
	}
}

func TestGenerateConfigMultiModuleMaven(t *testing.T) {
	det := &DetectedProject{
		ProjectName:       "my-platform",
		RootDir:           "/tmp/test",
		MultiModule:       true,
		ModuleNames:       []string{"api"},
		DeployableModules: []string{"api"},
		Types: []ProjectType{
			{Name: "springboot", BuildTool: "maven", Confidence: 95, Details: "Spring Boot with Maven"},
		},
	}

	config := GenerateConfig(det)
	if !searchString(config, "mvnw") {
		t.Error("expected mvnw in maven config")
	}
}

func TestGenerateConfigWithConfigDir(t *testing.T) {
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, "config"), 0755)

	det := &DetectedProject{
		ProjectName:       "test",
		RootDir:           tmpDir,
		HasConfigDir:      true,
		DeployableModules: []string{"test"},
		Types: []ProjectType{
			{Name: "generic", Confidence: 30, Details: "Generic"},
		},
	}

	config := GenerateConfig(det)
	if !searchString(config, "config_dir") {
		t.Error("expected config_dir in generated config")
	}
}

// --- Difference helper ---

func TestDifference(t *testing.T) {
	tests := []struct {
		a, b     []string
		expected int
	}{
		{[]string{"a", "b", "c"}, []string{"b"}, 2},
		{[]string{"a", "b"}, []string{"a", "b"}, 0},
		{[]string{"a"}, []string{}, 1},
		{[]string{}, []string{"a"}, 0},
	}

	for _, tt := range tests {
		result := difference(tt.a, tt.b)
		if len(result) != tt.expected {
			t.Errorf("difference(%v, %v) = %v, expected %d items", tt.a, tt.b, result, tt.expected)
		}
	}
}

// --- Java Gradle skips when already detected ---

func TestDetectJavaGradleSkipsWhenSpringBoot(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "build.gradle"), []byte(`
plugins {
    id 'org.springframework.boot' version '3.2.0'
}
`), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have springboot but not java
	javaCount := 0
	for _, pt := range det.Types {
		if pt.Name == "java" {
			javaCount++
		}
	}
	if javaCount > 0 {
		t.Error("expected Java detector to be skipped when Spring Boot detected")
	}
}

func TestDetectJavaMavenSkipsWhenSpringBoot(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "pom.xml"), []byte(`
<project>
    <parent>
        <artifactId>spring-boot-starter-parent</artifactId>
    </parent>
</project>
`), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	javaCount := 0
	for _, pt := range det.Types {
		if pt.Name == "java" {
			javaCount++
		}
	}
	if javaCount > 0 {
		t.Error("expected Java Maven detector to be skipped when Spring Boot detected")
	}
}

// --- Node build tool: yarn ---

func TestGenerateConfigNodeYarn(t *testing.T) {
	det := &DetectedProject{
		ProjectName:       "frontend",
		RootDir:           "/tmp/test",
		DeployableModules: []string{"frontend"},
		Types: []ProjectType{
			{Name: "node", BuildTool: "yarn", Confidence: 85, Details: "Node.js with yarn"},
		},
	}

	config := GenerateConfig(det)
	if !searchString(config, "yarn") {
		t.Error("expected yarn in config")
	}
}

func TestGenerateConfigNodePnpm(t *testing.T) {
	det := &DetectedProject{
		ProjectName:       "frontend",
		RootDir:           "/tmp/test",
		DeployableModules: []string{"frontend"},
		Types: []ProjectType{
			{Name: "node", BuildTool: "pnpm", Confidence: 85, Details: "Node.js with pnpm"},
		},
	}

	config := GenerateConfig(det)
	if !searchString(config, "pnpm") {
		t.Error("expected pnpm in config")
	}
}

func TestGenerateConfigPythonUV(t *testing.T) {
	det := &DetectedProject{
		ProjectName:       "api",
		RootDir:           "/tmp/test",
		DeployableModules: []string{"api"},
		Types: []ProjectType{
			{Name: "python", BuildTool: "uv", Confidence: 90, Details: "Python with uv"},
		},
	}

	config := GenerateConfig(det)
	if !searchString(config, "uv") {
		t.Error("expected uv in config")
	}
}

func TestGenerateConfigJavaMaven(t *testing.T) {
	det := &DetectedProject{
		ProjectName:       "api",
		RootDir:           "/tmp/test",
		DeployableModules: []string{"api"},
		Types: []ProjectType{
			{Name: "java", BuildTool: "maven", Confidence: 85, Details: "Java with Maven"},
		},
	}

	config := GenerateConfig(det)
	if !searchString(config, "mvnw") {
		t.Error("expected mvnw in config")
	}
}
