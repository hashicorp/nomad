import fs from 'fs'
import { promisify } from 'util'
import readdirp from 'readdirp'
import lineReader from 'line-reader'
import { safeLoad } from 'js-yaml'

export function readAllFrontMatter(dirPath) {
  return new Promise((resolve) => {
    const fm = []
    readdirp(dirPath, { fileFilter: '*.mdx' })
      .on('data', (entry) => {
        let lineNum = 0
        const content = []
        fm.push(
          new Promise((resolve2, reject2) => {
            lineReader.eachLine(
              entry.fullPath,
              (line) => {
                // if it has any content other than `---`, the file doesn't have front matter, so we close
                if (lineNum === 0 && !line.match(/^---$/)) return false
                // if it's not the first line and we have a bottom delimiter, exit
                if (lineNum !== 0 && line.match(/^---$/)) return false
                // now we read lines until we match the bottom delimiters
                content.push(line)
                // increment line number
                lineNum++
              },
              (err) => {
                if (err) return reject2(err)
                content.push(
                  `__resourcePath: "${dirPath.split('/').pop()}/${entry.path}"`
                )
                resolve2(safeLoad(content.slice(1).join('\n')), {
                  filename: entry.fullPath,
                })
              }
            )
          })
        )
      })
      .on('end', () => {
        Promise.all(fm).then((res) => resolve(res))
      })
  })
}

export async function readContent(filePath) {
  try {
    return (await promisify(fs.readFile)(filePath)).toString()
  } catch (error) {
    if (error.code === 'ENOENT') {
      return undefined
    }
    throw error
  }
}
