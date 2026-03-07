// AJAX helpers for reacting to posts (likes/dislikes)
document.addEventListener("DOMContentLoaded", () => {
    // handle all reaction forms anywhere on the page (posts and comments)
    document.querySelectorAll('form.inline-form').forEach(form => {
        form.addEventListener('submit', async e => {
            e.preventDefault();

            const formData = new FormData(form);
            const params = new URLSearchParams();
            for (const [k, v] of formData) {
                params.append(k, v);
            }

            try {
                const res = await fetch(form.action, {
                    method: 'POST',
                    body: params,
                    credentials: 'same-origin',
                    headers: {
                        'X-Requested-With': 'XMLHttpRequest'
                    }
                });

                if (!res.ok) {
                    throw new Error('Network response was not ok');
                }

                const data = await res.json();

                // update button text for this form
                const type = formData.get('type');
                const btn = form.querySelector('button');
                if (type === '1') {
                    btn.textContent = `👍 ${data.likes}`;
                } else if (type === '-1') {
                    btn.textContent = `👎 ${data.dislikes}`;
                }

                // also update sibling form (the opposite reaction)
                const otherForm = Array.from(form.parentElement.querySelectorAll('form.inline-form'))
                    .find(f => f !== form);
                if (otherForm) {
                    const otherType = otherForm.querySelector('input[name="type"]').value;
                    const otherBtn = otherForm.querySelector('button');
                    if (otherType === '1') {
                        otherBtn.textContent = `👍 ${data.likes}`;
                    } else if (otherType === '-1') {
                        otherBtn.textContent = `👎 ${data.dislikes}`;
                    }
                }

            } catch (err) {
                console.error('Failed to submit reaction', err);
            }
        });
    });
});
