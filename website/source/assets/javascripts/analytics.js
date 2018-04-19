document.addEventListener('turbolinks:load', function() {
  analytics.page()
  
    var m = el.href.match(/nomad_(.*?)_(.*?)_(.*?)\.zip/)
    return {
      event: 'Download',
      category: 'Button',
      label: 'Nomad | v' + m[1] + ' | ' + m[2] + ' | ' + m[3],
      version: m[1],
      os: m[2],
      architecture: m[3],
      product: 'nomad'
    }
  })
})
