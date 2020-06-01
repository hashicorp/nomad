import Controller from '@ember/controller';
import { computed } from '@ember/object';

export default Controller.extend({
  queryParams: {
    sortProperty: 'sort',
    sortDescending: 'desc',
  },

  sortProperty: 'Name',
  sortDescending: false,

  path: null,
  allocation: null,
  directoryEntries: null,
  isFile: null,
  stat: null,

  pathWithLeadingSlash: computed('path', function() {
    const path = this.path;

    if (path.startsWith('/')) {
      return path;
    } else {
      return `/${path}`;
    }
  }),
});
