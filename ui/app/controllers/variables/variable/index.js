import Controller from '@ember/controller';
import { action } from '@ember/object';
import { task } from 'ember-concurrency';
import messageForError from '../../../utils/message-from-adapter-error';
import { inject as service } from '@ember/service';
import { tracked } from '@glimmer/tracking';

export default class VariablesVariableIndexController extends Controller {
  @service router;

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
  @tracked
  isCodeView = false;

  toggleCodeView() {
    this.isCodeView = !this.isCodeView;
  }

  //#endregion Code View
}
