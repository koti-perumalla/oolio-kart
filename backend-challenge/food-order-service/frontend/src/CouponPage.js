import React from "react";
import UploadCoupons from "./UploadCoupons";
import Progress from "./Progress";

export default function CouponPage() {
  return (
    <div style={{ border: "1px solid #e5e7eb", borderRadius: 10, padding: 16 }}>
      <h2 style={{ marginTop: 0 }}>Promo Code Processing</h2>
      <UploadCoupons />
      <Progress />
    </div>
  );
}
