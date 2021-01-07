// The root folder for this documentation category is `pages/intro`
//
// - A string refers to the name of a file
// - A "category" value refers to the name of a directory
// - All directories must have an "index.mdx" file to serve as
//   the landing page for the category

export default [
  'use-cases',
  { category: 'vs', content: ['ecs', 'mesos', 'terraform'] },
  {
    category: 'getting-started',
    name: 'Getting Started',
    content: [
      {
        title: 'Overview',
        href: 'https://learn.hashicorp.com/collections/nomad/get-started',
      },
      {
        title: 'Running Nomad',
        href: 'https://learn.hashicorp.com/tutorials/nomad/get-started-run',
      },
      {
        title: 'Jobs',
        href: 'https://learn.hashicorp.com/tutorials/nomad/get-started-jobs',
      },
      {
        title: 'Clustering',
        href: 'https://learn.hashicorp.com/tutorials/nomad/get-started-cluster',
      },
      {
        title: 'Web UI',
        href: 'https://learn.hashicorp.com/tutorials/nomad/get-started-ui',
      },
      {
        title: 'Next Steps',
        href:
          'https://learn.hashicorp.com/tutorials/nomad/get-started-learn-more',
      },
    ],
  },
]
