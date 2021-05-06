module.exports = [
  {
    source: '/api/:splat((?!versioned-asset).*)',
    destination: '/api-docs/:splat',
  },
]
