import * as React from "react";
import { useNavigate } from "react-router-dom";
import { LinkFormDialog } from "@/features/links/link-form-dialog";

/** Route target for /app/links/new — opens the create dialog over the links list. */
export default function NewLinkPage() {
  const navigate = useNavigate();
  const [open, setOpen] = React.useState(true);

  return (
    <LinkFormDialog
      open={open}
      onOpenChange={(o) => {
        setOpen(o);
        if (!o) navigate("/app/links");
      }}
      onCreated={() => navigate("/app/links")}
    />
  );
}
