import Component from '@ember/component';
import { computed } from '@ember/object';

export default Component.extend({
  tagName: 'figure',
  classNames: 'image-file',
  'data-test-image-file': true,

  src: null,
  alt: null,
  size: null,

  // Set by updateImageMeta
  width: 0,
  height: 0,

  fileName: computed('src', function() {
    if (!this.src) return;
    return this.src.includes('/') ? this.src.match(/^.*\/(.*)$/)[1] : this.src;
  }),

  updateImageMeta(event) {
    const img = event.target;
    this.setProperties({
      width: img.naturalWidth,
      height: img.naturalHeight,
    });
  },
});
