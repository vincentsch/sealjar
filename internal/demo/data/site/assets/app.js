// Fills the opening hours after load, so a capture must wait for scripts.
document.addEventListener('DOMContentLoaded', function () {
  document.getElementById('hours').textContent = 'Open Monday to Saturday, 6:30 to 18:00';
});
