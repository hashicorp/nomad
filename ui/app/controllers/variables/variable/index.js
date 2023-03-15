import Controller from '@ember/controller';
import { set, action } from '@ember/object';
import { task } from 'ember-concurrency';
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
      this.flashMessages.add({
        title: 'Variable deleted',
        message: `${this.model.path} successfully deleted`,
        type: 'success',
        destroyOnClick: false,
        timeout: 5000,
      });
    } catch (err) {
      this.flashMessages.add({
        title: `Error deleting ${this.model.path}`,
        message: err,
        type: 'error',
        destroyOnClick: false,
        sticky: true,
      });
    }
  })
  deleteVariableFile;

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
