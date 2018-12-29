import { equal } from '@ember/object/computed';
import Component from '@ember/component';

export default Component.extend({
  classNames: ['job-diff'],
  classNameBindings: ['isEdited:is-edited', 'isAdded:is-added', 'isDeleted:is-deleted'],

  diff: null,

  verbose: true,

  isEdited: equal('diff.Type', 'Edited'),
  isAdded: equal('diff.Type', 'Added'),
  isDeleted: equal('diff.Type', 'Deleted'),
});
