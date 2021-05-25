# `@hashicorp/versioned-docs`

Welcome to the home of the tools and utilities necessary to power versioned docs for HashiCorp's products!

## Scripts

#### `versioned-docs-ingest <product> <contentDir>`

Kicks off the ingestion process based on the data preset in `version-manifest.json`. Any versions in the manifest which are not found in the database are marked for ingestion.

#### `versioned-docs-add-version <version> [ref=version] [slug=version] [display=version]`

Adds a new entry to the local version manifest file. `version` is a required argument, the rest are optional and default to the value of `version`.

## Methods

This package is broken into several separate entry points to ensure proper tree-shaking and to provide clear buckets of functionality.

### `@hashicorp/versioned-docs/ingest`

---

Exposes methods to execute the full ingestion process for a version of a product's documentation.

#### `extractDocuments(directory: string)`

Extracts document content from the specified directory. Returns a flat array of all content files.

#### `extractNavData(directory: string, version: Version)`

Extracts nav data from the specified directory. Returns the nav data in the updated JSON format.

#### `getVersionsToBeIngested(product: string)`

Determines which versions in `version-manifest.json` are not yet in the database and are eligible to be ingested.

#### `ingestDocumentationVersion({ product: string, version: Version, directory: string })`

Runs the full ingestion process for the specified product and version. Extracts the documents, applies transforms, and loads them into the database.

### `@hashicorp/versioned-docs/transforms`

---

Exposes transforms and the utilities to execute them on `VersionedDocument` objects.

#### `applyTransformToDocument(document: VersionedDocument, transform: string | Transform, rootDir: string)`

Applies a transform to a given document and updates the documents `mdxTransforms` list.

#### `BASE_TRANSFORMS: string[]`

An array of the transform IDs which are applied to all documents.

### `@hashicorp/versioned-docs/server`

---

Exposes methods to get versioned document data from the database in an application server environment.

#### `loadVersionListFromManifest()`

Returns a structured array of versions based on the data in `version-manifest.json`.

#### `loadVersionedNavData(product: string, basePath: string, version: string)`

Loads nav data for the provided product and version from the database.

#### `loadVersionedDocument(product: string, fullPath: string)`

Loads a versioned document from the database.

### `@hashicorp/versioned-docs/client`

---

Exposes methods to handle versioned document concerns on the client or within the runtime application.

#### `getVersionFromPath(path: string | string[] = [])`

Extracts the version from a path string or path segment array.
