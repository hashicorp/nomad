import Controller from '@ember/controller';
import { filterBy } from '@ember/object/computed';

export default Controller.extend({
  directories: filterBy('ls', 'IsDir'),
  files: filterBy('ls', 'IsDir', false),
});
