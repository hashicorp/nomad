export const ALERT_BANNER_ACTIVE = true

// https://github.com/hashicorp/web-components/tree/master/packages/alert-banner
export default {
  linkText: 'Learn more!',
  url: 'https://www.hashicorp.com/blog/announcing-general-availability-of-hashicorp-nomad-1-0',
  tag: 'ANNOUNCEMENT',
  text:
    'Nomad 1.0 is now generally available, which includes 5 major new features and many improvements.',
  // Set the `expirationDate prop with a datetime string (e.g. `2020-01-31T12:00:00-07:00`)
  // if you'd like the component to stop showing at or after a certain date
  expirationDate: '2021-02-08T09:00:00-07:00',
}
