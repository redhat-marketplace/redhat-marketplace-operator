export IMAGES=""
        ## getting image shas redhat-marketplace-operator
        shas="$(skopeo inspect docker://quay.io/rh-marketplace/redhat-marketplace-operator:$TAG --raw | jq -r '.manifests[].digest' | xargs)"
        #echo "found $shas for https://connect.redhat.com/projects/5e98b6fac77ce6fca8ac859c/images"
        for sha in $shas; do
        export IMAGES="--images https://connect.redhat.com/projects/5e98b6fac77ce6fca8ac859c/images,${sha},$TAG $IMAGES"
        done

        ## getting image shas redhat-marketplace-metric-state
        shas="$(skopeo inspect docker://quay.io/rh-marketplace/redhat-marketplace-metric-state:$TAG --raw | jq -r '.manifests[].digest' | xargs)"
        #echo "found $shas for https://connect.redhat.com/projects/5f36ea2f74cc50b8f01a838d/images"
        for sha in $shas; do
        export IMAGES="--images https://connect.redhat.com/projects/5f36ea2f74cc50b8f01a838d/images,${sha},$TAG $IMAGES"
        done

        ## getting image shas redhat-marketplace-reporter
        shas="$(skopeo inspect docker://quay.io/rh-marketplace/redhat-marketplace-reporter:$TAG --raw | jq -r '.manifests[].digest' | xargs)"
        #echo "found $shas for https://connect.redhat.com/projects/5e98b6fc32116b90fd024d06/images"
        for sha in $shas; do
        export IMAGES="--images https://connect.redhat.com/projects/5e98b6fc32116b90fd024d06/images,${sha},$TAG $IMAGES"
        done

        ## getting image shas redhat-marketplace-authcheck
        shas="$(skopeo inspect docker://quay.io/rh-marketplace/redhat-marketplace-authcheck:$TAG --raw | jq -r '.manifests[].digest' | xargs)"
        #echo "found $shas for https://connect.redhat.com/projects/5f62b71018e80cdc21edf22f/images"
        for sha in $shas; do
        export IMAGES="--images https://connect.redhat.com/projects/5f62b71018e80cdc21edf22f/images,${sha},$TAG $IMAGES"
        done


        echo $IMAGES
