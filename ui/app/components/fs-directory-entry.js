import Component from '@ember/component';
import { computed } from '@ember/object';

export default Component.extend({
  tagName: '',

  pathToEntry: computed('path', 'entry.Name', function() {
    const pathWithNoLeadingSlash = this.get('path').replace(/^\//, '');
    const name = this.get('entry.Name');

    return `${pathWithNoLeadingSlash}/${name}`;
  }),
});
