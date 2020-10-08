const path = require('path')
const { indexDocsContent } = require('@hashicorp/react-search/tools')

indexDocsContent({
  contentDir: path.join(process.cwd(), 'content'),
  globOptions: { ignore: path.join(process.cwd(), 'content/partials/**/*') },
})
