import { useListExtensionGitTokens, useRemoveExtensionGitToken, useSetExtensionGitToken } from "@/api/hooks/extensions.hooks"
import { Button, IconButton } from "@/components/ui/button"
import { Modal } from "@/components/ui/modal"
import { Separator } from "@/components/ui/separator"
import { TextInput } from "@/components/ui/text-input"
import React from "react"
import { BiTrash } from "react-icons/bi"
import { LuKeyRound } from "react-icons/lu"
import { toast } from "sonner"

type GitTokensModalProps = {
    children?: React.ReactElement
    open?: boolean
    onOpenChange?: (open: boolean) => void
}

export function GitTokensModal(props: GitTokensModalProps) {

    const {
        children,
        ...rest
    } = props

    const [repository, setRepository] = React.useState<string>("")
    const [token, setToken] = React.useState<string>("")

    const { data: tokens, isLoading } = useListExtensionGitTokens()
    const { mutate: setGitToken, isPending: isSaving } = useSetExtensionGitToken()
    const { mutate: removeGitToken, isPending: isRemoving } = useRemoveExtensionGitToken()

    function handleSave() {
        if (!repository || !token) {
            toast.warning("Please provide a repository and a token.")
            return
        }
        setGitToken({ repository, token }, {
            onSuccess: () => {
                setRepository("")
                setToken("")
            },
        })
    }

    return (
        <Modal
            title="Private repositories"
            trigger={children}
            contentClass="max-w-2xl"
            {...rest}
        >
            <div className="space-y-4">
                <p className="text-(--muted) text-sm">
                    Map a git repository to an access token to install and update extensions from private repositories.
                    The repository can be a full URL, <code>host/owner/repo</code>, <code>host/owner</code> or a bare host.
                    A token for <code>github.com/owner/repo</code> also covers raw and API URLs of that repository.
                </p>

                <div className="flex gap-2 items-end flex-wrap">
                    <TextInput
                        label="Repository"
                        placeholder="e.g. github.com/owner/repo"
                        value={repository}
                        onValueChange={setRepository}
                        fieldClass="flex-1 basis-52"
                    />
                    <TextInput
                        label="Token"
                        type="password"
                        placeholder="Access token"
                        value={token}
                        onValueChange={setToken}
                        fieldClass="flex-1 basis-52"
                    />
                    <Button
                        intent="primary-subtle"
                        leftIcon={<LuKeyRound className="text-lg" />}
                        loading={isSaving}
                        onClick={handleSave}
                    >
                        Save
                    </Button>
                </div>

                <Separator />

                {(!tokens?.length && !isLoading) && (
                    <p className="text-(--muted) text-sm text-center">
                        No tokens configured.
                    </p>
                )}

                {!!tokens?.length && (
                    <div className="space-y-2">
                        {tokens.map(t => (
                            <div key={t.repository} className="flex items-center gap-2 rounded-md border p-2 px-3">
                                <div className="flex-1 min-w-0">
                                    <p className="truncate font-medium">{t.repository}</p>
                                    <p className="text-(--muted) text-sm">{t.maskedToken}</p>
                                </div>
                                <IconButton
                                    icon={<BiTrash />}
                                    intent="alert-subtle"
                                    size="sm"
                                    loading={isRemoving}
                                    onClick={() => removeGitToken({ repository: t.repository })}
                                />
                            </div>
                        ))}
                    </div>
                )}
            </div>
        </Modal>
    )
}
