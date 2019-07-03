import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import moment from 'moment';

module('Unit | Controller | allocations/allocation/task/fs', function(hooks) {
  setupTest(hooks);

  test('it sorts with size-sorting falling back to name for directories', function(assert) {
    let controller = this.owner.lookup('controller:allocations/allocation/task/fs');

    controller.set('directoryEntries', [
      {
        Name: 'aaa-big-old-file',
        IsDir: false,
        Size: 19190000,
        ModTime: moment()
          .subtract(1, 'year')
          .format(),
      },
      {
        Name: 'mmm-small-mid-file',
        IsDir: false,
        Size: 1919,
        ModTime: moment()
          .subtract(6, 'month')
          .format(),
      },
      {
        Name: 'zzz-med-new-file',
        IsDir: false,
        Size: 191900,
        ModTime: moment().format(),
      },
      {
        Name: 'aaa-big-old-directory',
        IsDir: true,
        Size: 19190000,
        ModTime: moment()
          .subtract(1, 'year')
          .format(),
      },
      {
        Name: 'mmm-small-mid-directory',
        IsDir: true,
        Size: 1919,
        ModTime: moment()
          .subtract(6, 'month')
          .format(),
      },
      {
        Name: 'zzz-med-new-directory',
        IsDir: true,
        Size: 191900,
        ModTime: moment().format(),
      },
    ]);

    assert.deepEqual(controller.sortedDirectoryEntries.mapBy('Name'), [
      'aaa-big-old-directory',
      'mmm-small-mid-directory',
      'zzz-med-new-directory',
      'aaa-big-old-file',
      'mmm-small-mid-file',
      'zzz-med-new-file',
    ]);

    controller.setProperties({
      sortProperty: 'Name',
      sortDescending: true,
    });

    assert.deepEqual(controller.sortedDirectoryEntries.mapBy('Name'), [
      'zzz-med-new-file',
      'mmm-small-mid-file',
      'aaa-big-old-file',
      'zzz-med-new-directory',
      'mmm-small-mid-directory',
      'aaa-big-old-directory',
    ]);

    controller.setProperties({
      sortProperty: 'ModTime',
      sortDescending: false,
    });

    assert.deepEqual(controller.sortedDirectoryEntries.mapBy('Name'), [
      'aaa-big-old-directory',
      'mmm-small-mid-directory',
      'zzz-med-new-directory',
      'aaa-big-old-file',
      'mmm-small-mid-file',
      'zzz-med-new-file',
    ]);

    controller.setProperties({
      sortProperty: 'ModTime',
      sortDescending: true,
    });

    assert.deepEqual(controller.sortedDirectoryEntries.mapBy('Name'), [
      'zzz-med-new-file',
      'mmm-small-mid-file',
      'aaa-big-old-file',
      'zzz-med-new-directory',
      'mmm-small-mid-directory',
      'aaa-big-old-directory',
    ]);

    controller.setProperties({
      sortProperty: 'Size',
      sortDescending: false,
    });

    assert.deepEqual(
      controller.sortedDirectoryEntries.mapBy('Name'),
      [
        'aaa-big-old-directory',
        'mmm-small-mid-directory',
        'zzz-med-new-directory',
        'mmm-small-mid-file',
        'zzz-med-new-file',
        'aaa-big-old-file',
      ],
      'expected directories to be sorted by name and files to be sorted by ascending size'
    );

    controller.setProperties({
      sortProperty: 'Size',
      sortDescending: true,
    });

    assert.deepEqual(
      controller.sortedDirectoryEntries.mapBy('Name'),
      [
        'aaa-big-old-file',
        'zzz-med-new-file',
        'mmm-small-mid-file',
        'zzz-med-new-directory',
        'mmm-small-mid-directory',
        'aaa-big-old-directory',
      ],
      'expected files to be sorted by descending size and directories to be sorted by descending name'
    );
  });
});
