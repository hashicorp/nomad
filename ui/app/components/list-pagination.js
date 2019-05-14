import Component from '@ember/component';
import { computed } from '@ember/object';

export default Component.extend({
  source: computed(() => []),
  size: 25,
  page: 1,
  spread: 2,

  startsAt: computed('size', 'page', function() {
    return (this.page - 1) * this.size + 1;
  }),

  endsAt: computed('source.[]', 'size', 'page', function() {
    return Math.min(this.page * this.size, this.get('source.length'));
  }),

  lastPage: computed('source.[]', 'size', function() {
    return Math.ceil(this.get('source.length') / this.size);
  }),

  pageLinks: computed('source.[]', 'page', 'spread', function() {
    const { spread, page, lastPage } = this;

    // When there is only one page, don't bother with page links
    if (lastPage === 1) {
      return [];
    }

    const lowerBound = Math.max(1, page - spread);
    const upperBound = Math.min(lastPage, page + spread) + 1;

    return Array(upperBound - lowerBound)
      .fill(null)
      .map((_, index) => ({
        pageNumber: lowerBound + index,
      }));
  }),

  list: computed('source.[]', 'page', 'size', function() {
    const size = this.size;
    const start = (this.page - 1) * size;
    return this.source.slice(start, start + size);
  }),
});
