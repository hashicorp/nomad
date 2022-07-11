import Controller from '@ember/controller';
import { set, action } from '@ember/object';
import { task } from 'ember-concurrency';
import messageForError from '../../../utils/message-from-adapter-error';
import { inject as service } from '@ember/service';
import { tracked } from '@glimmer/tracking';

export default class VariablesVariableIndexController extends Controller {
  queryParams = ['view'];

  @service router;
  queryParams = ['sortProperty', 'sortDescending'];

  @tracked sortProperty = 'key';
  @tracked sortDescending = true;

  get sortedKeyValues() {
    const sorted = this.model.keyValues.sortBy(this.sortProperty);
    return this.sortDescending ? sorted : sorted.reverse();
  }

  @tracked
  error = null;

  @tracked isDeleting = false;

  @action
  onDeletePrompt() {
    this.isDeleting = true;
  }

  @action
  onDeleteCancel() {
    this.isDeleting = false;
  }

  @task(function* () {
    try {
      yield this.model.deleteRecord();
      yield this.model.save();
      if (this.model.parentFolderPath) {
        this.router.transitionTo('variables.path', this.model.parentFolderPath);
      } else {
        this.router.transitionTo('variables');
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

  //#region Code View
  /**
   * @type {"table" | "json"}
   */
  @tracked
  view = 'table';

  toggleView() {
    if (this.view === 'table') {
      this.view = 'json';
    } else {
      this.view = 'table';
    }
  }
  //#endregion Code View

  get shouldShowLinkedEntities() {
    return (
      this.model.pathLinkedEntities?.job ||
      this.model.pathLinkedEntities?.group ||
      this.model.pathLinkedEntities?.task
    );
  }

  toggleRowVisibility(kv) {
    set(kv, 'isVisible', !kv.isVisible);
  }
}
