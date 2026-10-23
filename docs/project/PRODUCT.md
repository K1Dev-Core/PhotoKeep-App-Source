# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

People using the desktop app to back up and find image files in folders on their computer. This audience is inferred from the existing app workflows and the user's request.

## Product Purpose

PhotoKeep copies images from a chosen source folder to a backup folder, keeps the local catalog current, and helps users find images by filename, description, or tags.

## Positioning

PhotoKeep combines local image backup and a searchable image catalog in one desktop app. Image descriptions and tags can be added or edited to improve search.

## Operating Context

Users choose source and destination folders, back up images, refresh the catalog when files change, search and filter the image library, inspect or edit image details, and optionally analyze selected images with the configured AI image-analysis server.

## Capabilities and Constraints

- The app uses Wails, Go, Nuxt, and SQLite.
- The desktop app is expected to work on Windows and macOS.
- Backup progress is visible while files are copied.
- Search supports image filenames, descriptions, and tags; tags are also shown as a tag cloud.
- The AI server access token can be managed in settings, and selected images can be analyzed locally through a resumable queue.
- Preserve all existing app behavior while changing the interface.
- A local image-analysis service hosted on a separate Windows laptop is a planned integration; its API contract and URL are not yet available.

## Brand Commitments

- Keep the product name PhotoKeep.
- The user now wants a photo-first desktop layout based on their supplied Pinterest-style reference, with a left navigation rail, broad search bar, compact utility actions, neutral charcoal primary actions, and a masonry image gallery.
- Keep the interface simple and uncluttered, use concise Thai copy and recognizable icons, and follow the reference’s white, warm-gray, and charcoal palette. Use rounded image tiles modestly while keeping controls compact.

## Evidence on Hand

- Existing implementation and workflows in this repository.
- User-provided interface references earlier in the conversation; no production brand assets were supplied.

## Product Principles

- Make backing up and refreshing the image library immediately understandable.
- Let the images remain the main content of the gallery.
- Keep controls concise and easy to recognize.
- Preserve local backup and catalog behavior when improving the interface.
