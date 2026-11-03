# Razorpay Test-Mode Verification & Gateway Testing Guide

This guide describes how to configure, test, and verify Razorpay test-mode payments and webhooks locally on the Adaptive DSA Interview Preparation Platform.

---

## 1. Prerequisites & Test Credentials

1. **Razorpay Dashboard Account**:
   - Log in to [Razorpay Dashboard](https://dashboard.razorpay.com).
   - Ensure the top-left toggle is set to **Test Mode**.
2. **Generate Test API Keys**:
   - Navigate to **Account & Settings** -> **API Keys**.
   - Click **Generate Test Key**.
   - Copy `Key ID` (starts with `rzp_test_...`) and `Key Secret`.
3. **Configure Local Environment (`.env`)**:
   ```env
   RAZORPAY_KEY_ID=rzp_test_YOUR_KEY_ID
   RAZORPAY_KEY_SECRET=YOUR_KEY_SECRET
   RAZORPAY_WEBHOOK_SECRET=YOUR_WEBHOOK_SECRET_STRING_32_BYTES
   ```

---

## 2. Webhook Tunneling for Local Testing

Because Razorpay servers need to deliver HTTP POST webhooks to your local machine, use a secure webhook tunnel (e.g. ngrok):

```bash
# Expose port 80 (or 8080 if running backend directly)
ngrok http 80
```
- Copy the public HTTPS forwarding URL: `https://your-tunnel-id.ngrok-free.app`.

### Configure Webhook in Razorpay Dashboard:
1. In the Razorpay Dashboard, go to **Account & Settings** -> **Webhooks** -> **Add New Webhook**.
2. **Webhook URL**: `https://your-tunnel-id.ngrok-free.app/api/v1/payments/webhook`
3. **Secret**: Enter the exact secret string you set for `RAZORPAY_WEBHOOK_SECRET` in `.env`.
4. **Active Events**: Check the following events:
   - `payment.captured`
   - `order.paid`
   - `payment.failed`
   - `payment.authorized`
5. Click **Create Webhook**.

---

## 3. End-to-End Test Purchase Walkthrough

1. **Launch the Platform Locally**:
   ```bash
   docker compose -f docker-compose.local.yml up --build
   ```
2. **Open the Credits Page**:
   - Open [http://localhost:3000/credits](http://localhost:3000/credits) in your browser.
   - Log in or register an account.
3. **Select a Package**:
   - Click **Purchase** on the **Pro Pack** (₹249 for 150 credits).
   - The frontend calls `POST /api/v1/payments/orders` with `{"package_id": "pkg_pro"}`.
   - The backend enforces authoritative pricing (24,900 paise) and returns a Razorpay Order ID (`order_...`).
4. **Complete Checkout Modal**:
   - Razorpay's checkout modal opens automatically.
   - Select **Card** payment.
   - Enter standard test card details:
     - **Card Number**: `4111 1111 1111 1111`
     - **Expiry**: `12/30`
     - **CVV**: `123`
   - On the mock bank authentication screen:
     - Click **Success** or enter OTP `333333`.
5. **Observe Verification & Credit Granting**:
   - The frontend sends the payment response to `POST /api/v1/payments/verify`.
   - The backend validates the HMAC-SHA256 signature and authoritatively validates `status == "captured"`.
   - The order transitions to `PAID`, and 150 credits are added to the user's ledger and balance.
   - The UI automatically refreshes the balance badge and adds a transaction entry to the **Credit Ledger** and **Payment History** tabs.

---

## 4. Testing Failure and Interruption Scenarios

### Scenario A: Cancelled / Abandoned Checkout
1. Open the checkout modal for any package.
2. Click the `X` button to close the modal before completing payment.
3. *Expected Result*: The UI returns cleanly to the package selection view with an informational notification. No credits are granted, and no order status is updated to `PAID`.

### Scenario B: Payment Failure (Declined Card)
1. In the Razorpay modal, select **Card**.
2. On the OTP / Bank Authentication page, click **Failure** or enter OTP `111111`.
3. *Expected Result*: Razorpay triggers a `payment.failed` webhook. The order updates to `FAILED`. Zero credits are granted.

### Scenario C: Out-of-Order Webhook Safety
1. Complete a successful payment.
2. In the Razorpay Webhook logs, manually replay an earlier `payment.failed` event for the same order.
3. *Expected Result*: The backend recognizes the order is already in `PAID` status. The state transition guard rejects the downgrade. The order remains `PAID` and credit balance is untouched.
