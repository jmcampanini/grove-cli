function grc --description "Grove create - create branch and worktree"
    set -l output (grove create "$argv")
    if test $status -eq 0
        if command -q z
            z $output
        else
            cd $output
        end
    else
        echo $output
        return 1
    end
end
