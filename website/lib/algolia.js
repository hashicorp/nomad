import search from 'algoliasearch'
import insights from 'search-insights' //  click and conversion analytics methods are bundled separately from the main search client

const algoliaClient = search(
  process.env.ALGOLIA_APP_ID,
  process.env.ALGOLIA_SEARCH_ONLY_API_KEY
)

export const searchClient = {
  search(requests) {
    return algoliaClient.search(
      requests.map((request) => {
        //  instantsearch fires an empty query on page load to ensure results are immediately available
        //  we exclude that result from our analytics to keep our clickthrough rate clean
        //  ref: https://www.algolia.com/doc/guides/getting-insights-and-analytics/search-analytics/out-of-the-box-analytics/how-to/how-to-remove-empty-search-from-analytics/
        if (!request.params.query || request.params.query.length === 0) {
          request.params.analytics = false
        }

        return request
      })
    )
  },
}

export function initAlgoliaInsights() {
  insights.init({
    appId: process.env.ALGOLIA_APP_ID,
    apiKey: process.env.ALGOLIA_SEARCH_ONLY_API_KEY,
  })
}

export function logClick(hit) {
  return insights.clickedObjectIDsAfterSearch({
    eventName: 'CLICK_HIT',
    index: process.env.ALGOLIA_INDEX,
    queryID: hit.__queryID,
    objectIDs: [hit.objectID],
    positions: [hit.__position],
  })
}

export function logConversion({ queryID, objectIDs }) {
  insights.convertedObjectIDsAfterSearch({
    eventName: 'CONVERT_HIT',
    index: process.env.ALGOLIA_INDEX,
    queryID,
    objectIDs,
  })
}
