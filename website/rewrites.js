module.exports = [
  { source: '/api/:splat*', destination: '/api-docs/:splat*' },
  { source: '/test-rewrite/:splat*', destination: '/api-docs/:splat*' },
]
