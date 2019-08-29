import Component from '@ember/component';
import { computed } from '@ember/object';
import { isEmpty } from '@ember/utils';

export default Component.extend({
  tagName: '',

  pathToEntry: computed('path', 'entry.Name', function() {
    const pathWithNoLeadingSlash = this.get('path').replace(/^\//, '');
    const name = encodeURIComponent(this.get('entry.Name'));

    if (isEmpty(pathWithNoLeadingSlash)) {
      return name;
    } else {
      return `${pathWithNoLeadingSlash}/${name}`;
    }
  }),
});
