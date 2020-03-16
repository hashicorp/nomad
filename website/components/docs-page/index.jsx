import { useEffect } from 'react'
import DocsSidenav from '@hashicorp/react-docs-sidenav'
import Content from '@hashicorp/react-content'
import InlineSvg from '@hashicorp/react-inline-svg'
import githubIcon from './img/github-icon.svg?include'
import Link from 'next/link'
import Head from 'next/head'
import HashiHead from '@hashicorp/react-head'

export default function DocsPage({
  children,
  path,
  orderData,
  frontMatter,
  category,
  pageMeta
}) {
  // TEMPORARY (https://app.asana.com/0/1100423001970639/1160656182754009)
  useEffect(() => {
    const node = document.querySelector('#inner')
    if (!node) return
    return temporary_injectJumpToSection(node)
  }, [])

  return (
    <div id="p-docs">
      <HashiHead
        is={Head}
        title={`${pageMeta.page_title} | Nomad by HashiCorp`}
        description={pageMeta.description}
      />
      <div className="content-wrap g-container">
        <div id="sidebar" role="complementary">
          <div className="nav docs-nav">
            <DocsSidenav
              currentPage={path}
              category={category}
              order={orderData}
              data={frontMatter}
              Link={Link}
              product="nomad"
            />
          </div>
        </div>

        <div id="inner" role="main">
          <Content product="nomad" content={children} />
        </div>
      </div>
      <div id="edit-this-page" className="g-container">
        <a
          href={`https://github.com/hashicorp/nomad/blob/master/website/pages/${pageMeta.__resourcePath}`}
        >
          <InlineSvg src={githubIcon} />
          <span>Edit this page</span>
        </a>
      </div>
    </div>
  )
}

export async function getInitialProps({ asPath }) {
  return { path: asPath }
}

// This is terrible, very non-idiomatic code. It is here temporarily to enable this feature
// while the team works on a more permanent solution. Please do not ever write code like this.
// Ticket to fix this: https://app.asana.com/0/1100423001970639/1160656182754009
function temporary_injectJumpToSection(node) {
  const root = node.children[0]
  const firstH1 = root.querySelector('h1')
  const otherH2s = [].slice.call(root.querySelectorAll('h2')) // NodeList -> array

  // if there's no h1 or less than 3 h2s, don't render jump to section
  if (!firstH1) return
  if (otherH2s.length < 3) return

  const headlines = otherH2s.map(h2 => {
    // slice removes the anchor link character
    return { id: h2.id, text: h2.innerText.slice(1) }
  })

  // build the html
  const html = `
    <span class="trigger g-type-label">
      Jump to Section
      <svg width="9" height="5" xmlns="http://www.w3.org/2000/svg"><path d="M8.811 1.067a.612.612 0 0 0 0-.884.655.655 0 0 0-.908 0L4.5 3.491 1.097.183a.655.655 0 0 0-.909 0 .615.615 0 0 0 0 .884l3.857 3.75a.655.655 0 0 0 .91 0l3.856-3.75z" fill-rule="evenodd"/></svg>
    </span>
    <ul class="dropdown">
      ${headlines
        .map(h => `<li><a href="#${h.id}">${h.text}</a></li>`)
        .join('')}
    </ul>`
  const el = document.createElement('div')
  el.innerHTML = html
  el.id = 'jump-to-section'

  // attach event listeners to make the dropdown work
  const trigger = el.querySelector('.trigger')
  const dropdown = el.querySelector('.dropdown')
  const body = document.body
  const triggerEvent = e => {
    e.stopPropagation()
    dropdown.classList.toggle('active')
  }
  const clickOutsideEvent = () => dropdown.classList.remove('active')
  const clickInsideDropdownEvent = e => e.stopPropagation()
  trigger.addEventListener('click', triggerEvent)
  body.addEventListener('click', clickOutsideEvent)
  dropdown.addEventListener('click', clickInsideDropdownEvent)

  // inject the html after the first h1
  firstH1.parentNode.insertBefore(el, firstH1.nextSibling)

  // adjust the h1 margin
  firstH1.classList.add('has-jts')

  // cleanup function removes listeners on unmount
  return function cleanup() {
    trigger.removeEventListener('click', triggerEvent)
    body.removeEventListener('click', clickOutsideEvent)
    dropdown.removeEventListener('click', clickInsideDropdownEvent)
  }
}
