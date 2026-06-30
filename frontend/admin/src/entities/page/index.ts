export {
  listPages,
  getPage,
  createPage,
  updatePage,
  deletePage,
  verifyDomain,
  listAllowedEmails,
  addAllowedEmail,
  deleteAllowedEmail,
} from "./api";
export type {
  AllowedEmail,
  DomainStatus,
  StatusPage,
  StatusPageCreate,
  StatusPageUpdate,
} from "@/shared/api";
