package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInspectWebExperienceFailsForbiddenCopyAndTransitionEvents(t *testing.T) {
	root := t.TempDir()
	writeExperienceFixture(t, root, "src/App.vue", `
<template><button>开启一段奇妙旅程</button></template>
<script>element.addEventListener('transitionend', finish)</script>
<style>.panel { transition: opacity 200ms; } @keyframes drift { from { opacity: .4 } }</style>
`)

	result, err := inspectWebExperience(root)
	if err != nil {
		t.Fatal(err)
	}
	if result["ok"] != false {
		t.Fatalf("expected experience inspection to fail: %#v", result)
	}
	checks := experienceChecksByID(result)
	if checks["copy.forbidden"].Status != "fail" {
		t.Fatalf("forbidden copy was not detected: %#v", checks["copy.forbidden"])
	}
	if checks["motion.animation-events"].Status != "fail" {
		t.Fatalf("transitionend was not detected: %#v", checks["motion.animation-events"])
	}
	if checks["motion.reduced-motion"].Status != "fail" {
		t.Fatalf("missing reduced motion was not detected: %#v", checks["motion.reduced-motion"])
	}
}

func TestInspectWebExperiencePassesIntentionalMotion(t *testing.T) {
	root := t.TempDir()
	writeExperienceFixture(t, root, "src/App.vue", `
<template><button aria-label="确认并发送">确认并发送</button></template>
<script>requestAnimationFrame(draw)</script>
<style>
.panel { transition: opacity 240ms; }
@keyframes drift { from { transform: translateY(0) } to { transform: translateY(4px) } }
@media (prefers-reduced-motion: reduce) { .panel, .drift { animation: none; transition-duration: 1ms; } }
</style>
`)

	result, err := inspectWebExperience(root)
	if err != nil {
		t.Fatal(err)
	}
	if result["ok"] != true {
		encoded, _ := os.ReadFile(filepath.Join(root, "src", "App.vue"))
		t.Fatalf("expected experience inspection to pass: %#v source=%s", result, encoded)
	}
	checks := experienceChecksByID(result)
	if checks["motion.live-signals"].Status != "pass" {
		t.Fatalf("live signals were not detected: %#v", checks["motion.live-signals"])
	}
}

func TestInspectWebExperienceSkipsGeneratedAndTestSources(t *testing.T) {
	root := t.TempDir()
	writeExperienceFixture(t, root, "src/App.vue", `
<template><button>确认并发送</button></template>
<style>@keyframes drift {} .x { transition: none } @media (prefers-reduced-motion: reduce) { .x { animation: none } }</style>
`)
	writeExperienceFixture(t, root, "node_modules/pkg/index.js", `button('开启一段奇妙旅程'); transitionend`)
	writeExperienceFixture(t, root, "src/App.spec.ts", `expect('开启一段奇妙旅程')`)

	result, err := inspectWebExperience(root)
	if err != nil {
		t.Fatal(err)
	}
	checks := experienceChecksByID(result)
	if checks["copy.forbidden"].Count != 0 || checks["motion.animation-events"].Count != 0 {
		t.Fatalf("generated or test source was scanned: %#v", result)
	}
}

func experienceChecksByID(result map[string]any) map[string]experienceCheck {
	checks := make(map[string]experienceCheck)
	for _, raw := range result["checks"].([]experienceCheck) {
		checks[raw.ID] = raw
	}
	return checks
}

func writeExperienceFixture(t *testing.T, root, relative, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestInspectWebExperienceRejectsGenericErrors(t *testing.T) {
	root := t.TempDir()
	writeExperienceFixture(t, root, "src/App.vue", `
<template><button>重新开始</button></template>
<script>showError('系统异常')</script>
<style>.x { transition: opacity 180ms } @media (prefers-reduced-motion: reduce) { .x { animation: none } }</style>
<script>requestAnimationFrame(draw)</script>
`)

	result, err := inspectWebExperience(root)
	if err != nil {
		t.Fatal(err)
	}
	checks := experienceChecksByID(result)
	if checks["copy.error-states"].Status != "fail" {
		t.Fatalf("generic error copy was not detected: %#v", checks["copy.error-states"])
	}
}

func TestInspectWebExperienceWarnsLongTransitions(t *testing.T) {
	root := t.TempDir()
	writeExperienceFixture(t, root, "src/App.vue", `
<template><button>确认并发送</button></template>
<script>requestAnimationFrame(draw)</script>
<style>.slow { transition: opacity 1200ms; } @keyframes drift {} @media (prefers-reduced-motion: reduce) { .slow { animation: none } }</style>
`)

	result, err := inspectWebExperience(root)
	if err != nil {
		t.Fatal(err)
	}
	checks := experienceChecksByID(result)
	if checks["motion.transition-timing"].Status != "warn" || checks["motion.transition-timing"].Count != 1 {
		t.Fatalf("long transition was not reported: %#v", checks["motion.transition-timing"])
	}
}
