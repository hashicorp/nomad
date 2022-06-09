import Controller from '@ember/controller';
import { task } from 'ember-concurrency';
import messageForError from '../../../utils/message-from-adapter-error';
import { inject as service } from '@ember/service';
import { tracked } from '@glimmer/tracking';
import PathTree, { pathToObject } from 'nomad-ui/utils/path-tree';

export default class VariablesVariableIndexController extends Controller {
  @service router;

  @tracked
  error = null;

  @task(function* () {
    try {
      yield this.model.deleteRecord();
      yield this.model.save();
      console.log('what i got', this.model);
      console.log('and path transition if', pathToObject(this.model.path).path);
      const parentPath = pathToObject(this.model.path).path;
      console.log({ parentPath });
      // TODO: work on this conditional transition to parent path

      if (this.modelFor('variables').pathTree.findPath(parentPath)) {
        this.router.transitionTo('variables.path', parentPath);
      } else {
        this.router.transitionTo('variables.index');
      }
      // TODO: alert the user that the variable was successfully deleted
    } catch (err) {
      this.error = {
        title: 'Could Not Delete Variable',
        description: messageForError(err, 'delete secure variables'),
      };
    }
  })
  deleteVariableFile;

  onDismissError() {
    this.error = null;
  }
}
