# Nomad Website

[![Netlify Status](https://img.shields.io/netlify/57d01198-7abc-4c39-a89b-386a2fe7de09?style=flat-square)](https://app.netlify.com/sites/nomad-website/deploys)

This subdirectory contains the entire source for the [Nomad Website](https://nomadproject.io/). This is a [NextJS](https://nextjs.org/) project, which builds a static site from these source files.

## Contributions Welcome!

If you find a typo or you feel like you can improve the HTML, CSS, or JavaScript, we welcome contributions. Feel free to open issues or pull requests like any normal GitHub project, and we'll merge it in 🚀

## Running the Site Locally

The website can be run locally through node.js or Docker. If you choose to run through Docker, everything will be a little bit slower due to the additional overhead, so for frequent contributors it may be worth it to use node. Also if you are a vim user, it's also worth noting that vim's swapfile usage can cause issues for the live reload functionality. In order to avoid these issues, make sure you have run `:set backupcopy=yes` within vim.

### With Docker

Running the site locally is simple. Provided you have Docker installed, clone this repo, run `make`, and then visit `http://localhost:3000`.

The docker image is pre-built with all the website dependencies installed, which is what makes it so quick and simple, but also means if you need to change dependencies and test the changes within Docker, you'll need a new image. If this is something you need to do, you can run `make build-image` to generate a local Docker image with updated dependencies, then `make website-local` to use that image and preview.

### With Node

If your local development environment has a supported version (v10.0.0+) of [node installed](https://nodejs.org/en/) you can run:

- `npm install`
- `npm start`

and then visit `http://localhost:3000`.

If you pull down new code from github, you should run `npm install` again. Otherwise, there's no need to re-run `npm install` each time the site is run, you can just run `npm start` to get it going.

## Editing Content

Documentation content is written in [Markdown](https://www.markdownguide.org/cheat-sheet/) and you'll find all files listed under the `/pages` directory.

To create a new page with Markdown, create a file ending in `.mdx` in the `pages/` directory. The path in the pages directory will be the URL route. For example, `pages/hello/world.mdx` will be served from the `/hello/world` URL.

This file can be standard Markdown and also supports [YAML frontmatter](https://middlemanapp.com/basics/frontmatter/). YAML frontmatter is optional, there are defaults for all keys.

```yaml
---
title: 'My Title'
description: "A thorough, yet succinct description of the page's contents"
---

```

The significant keys in the YAML frontmatter are:

- `title` `(string)` - This is the title of the page that will be set in the HTML title.
- `description` `(string)` - This is a description of the page that will be set in the HTML description.

> ⚠️Since `api` is a reserved directory within NextJS, all `/api/**` pages are listed under the `/pages/api-docs` path.

### Editing Sidebars

The structure of the sidebars are controlled by files in the [`/data` directory](data).

- Edit [this file](data/docs-navigation.js) to change the **docs** sidebar
- Edit [this file](data/api-navigation.js) to change the **api docs** sidebar

To nest sidebar items, you'll want to add a new `category` key/value accompanied by the appropriate embedded `content` values.

- `category` values will be **directory names** within the `pages` directory
- `content` values will be **file names** within their appropriately nested directory.

### Creating New Pages

There is currently a small bug with new page creation - if you create a new page and link it up via subnav data while the server is running, it will report an error saying the page was not found. This can be resolved by restarting the server.

### Changing the Release Version

To change the version of Nomad displayed for download on the website, head over to `data/version.js` and change the number there. It's important to note that the version number must match a version that has been released and is live on `releases.hashicorp.com` -- if it does not, the website will be unable to fetch links to the binaries and will not compile. So this version number should be changed _only after a release_.

### Displaying a Prerelease

If there is a prerelease of any type that should be displayed on the downloads page, this can be done by editing `pages/downloads/index.jsx`. By default, the download component might look something like this:

```jsx
<ProductDownloader
  product="Nomad"
  version={VERSION}
  downloads={downloadData}
  community="/resources"
/>
```

To add a prerelease, an extra `prerelease` property can be added to the component as such:

```jsx
<ProductDownloader
  product="Nomad"
  version={VERSION}
  downloads={downloadData}
  community="/resources"
  prerelease={{
    type: 'release candidate', // the type of prerelease: beta, release candidate, etc.
    name: 'v1.0.0', // the name displayed in text on the website
    version: '1.0.0-rc1' // the actual version tag that was pushed to releases.hashicorp.com
  }}
/>
```

This configuration would display something like the following text on the website, emphasis added to the configurable parameters:

```
A {{ release candidate }} for Nomad {{ v1.0.0 }} is available! The release can be <a href='https://releases.hashicorp.com/nomad/{{ 1.0.0-rc1 }}'>downloaded here</a>.
```

You may customize the parameters in any way you'd like. To remove a prerelease from the website, simply delete the `prerelease` paremeter from the above component.

### Markdown Enhancements

There are several custom markdown plugins that are available by default that enhance standard markdown to fit our use cases. This set of plugins introduces a couple instances of custom syntax, and a couple specific pitfalls that are not present by default with markdown, detailed below:

- If you see the symbols `~>`, `->`, `=>`, or `!>`, these represent [custom alerts](https://github.com/hashicorp/remark-plugins/tree/master/plugins/paragraph-custom-alerts#paragraph-custom-alerts). These render as colored boxes to draw the user's attention to some type of aside.
- If you see `@include '/some/path.mdx'`, this is a [markdown include](https://github.com/hashicorp/remark-plugins/tree/master/plugins/include-markdown#include-markdown-plugin). It's worth noting as well that all includes resolve from `website/pages/partials` by default.
- If you see `# Headline ((#slug))`, this is an example of an [anchor link alias](https://github.com/hashicorp/remark-plugins/tree/je.anchor-link-adjustments/plugins/anchor-links#anchor-link-aliases). It adds an extra permalink to a headline for compatibility and is removed from the output.
- Due to [automatically generated permalinks](https://github.com/hashicorp/remark-plugins/tree/je.anchor-link-adjustments/plugins/anchor-links#anchor-links), any text changes to _headlines_ or _list items that begin with inline code_ can and will break existing permalinks. Be very cautious when changing either of these two text items.

  Headlines are fairly self-explanitory, but here's an example of how list items that begin with inline code look.

  ```markdown
  - this is a normal list item
  - `this` is a list item that begins with inline code
  ```

  Its worth noting that _only the inline code at the beginning of the list item_ will cause problems if changed. So if you changed the above markup to...

  ```markdown
  - lsdhfhksdjf
  - `this` jsdhfkdsjhkdsfjh
  ```

  ...while it perhaps would not be an improved user experience, no links would break because of it. The best approach is to **avoid changing headlines and inline code at the start of a list item**. If you must change one of these items, make sure to tag someone from the digital marketing development team on your pull request, they will help to ensure as much compatibility as possible.

## Deployment

This website is hosted on Netlify and configured to automatically deploy anytime you push code to the `stable-website` branch. Any time a pull request is submitted that changes files within the `website` folder, a deployment preview will appear in the github checks which can be used to validate the way docs changes will look live. Deployments from `stable-website` will look and behave the same way as deployment previews.
