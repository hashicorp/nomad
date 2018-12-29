import ApplicationSerializer from './application';

export default ApplicationSerializer.extend({
  attrs: {
    message: 'DisplayMessage',
  },

  normalize(typeHash, hash) {
    // Time is in the form of nanoseconds since epoch, but JS dates
    // only understand time to the millisecond precision. So store
    // the time (precise to ms) as a date, and store the remaining ns
    // as a number to deal with when it comes up.
    hash.TimeNanos = hash.Time % 1000000;
    hash.Time = Math.floor(hash.Time / 1000000);

    return this._super(typeHash, hash);
  },
});
