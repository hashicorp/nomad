require('dotenv').config()

const algoliasearch = require('algoliasearch')
const glob = require('glob')
const path = require('path')
const matter = require('gray-matter')

main()

async function main() {
  const pagesFolder = path.join(__dirname, '../pages')

  //  grab all search-indexable content and format for algolia
  const searchObjects = glob
    .sync(path.join(pagesFolder, '**/*.mdx'))
    .map((fullPath) => {
      const { content, data } = matter.read(fullPath)

      //  get its path relative to `pages`
      const __resourcePath = fullPath.replace(`${pagesFolder}/`, '')

      //  use clean URL for algolia id
      const objectID = __resourcePath.replace('.mdx', '')

      return {
        ...data,
        content,
        __resourcePath,
        objectID,
      }
    })

  try {
    await indexSearchContent(searchObjects)
  } catch (e) {
    console.error(e)
    process.exit(1)
  }
}

async function indexSearchContent(objects) {
  const {
    ALGOLIA_APP_ID: appId,
    ALGOLIA_INDEX: index,
    ALGOLIA_API_KEY: apiKey,
  } = process.env

  if (!apiKey || !appId || !index) {
    throw new Error(
      `[*** Algolia Search Indexing Error ***] Received: ALGOLIA_API_KEY=${apiKey} ALGOLIA_APP_ID=${appId} ALGOLIA_INDEX=${index} \n Please ensure all Algolia Search-related environment vars are set in CI settings.`
    )
  }

  console.log(`updating ${objects.length} indices...`)

  try {
    const searchClient = algoliasearch(appId, apiKey)
    const searchIndex = searchClient.initIndex(index)

    await searchIndex.partialUpdateObjects(objects, {
      createIfNotExists: true,
    })

    //  remove indices for items that aren't included in the new batch
    const newObjectIds = objects.map(({ objectID }) => objectID)
    let staleObjects = []

    await searchIndex.browseObjects({
      query: '',
      batch: (batch) => {
        staleObjects = staleObjects.concat(
          batch.filter(({ objectID }) => !newObjectIds.includes(objectID))
        )
      },
    })

    const staleIds = staleObjects.map(({ objectID }) => objectID)

    if (staleIds.length > 0) {
      console.log(`deleting ${staleIds.length} stale indices:`)
      console.log(staleIds)

      await searchIndex.deleteObjects(staleIds)
    }

    console.log('done')
    process.exit(0)
  } catch (error) {
    throw new Error(error)
  }
}
