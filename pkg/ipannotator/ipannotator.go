package ipannotator

import (
	"fmt"

	"github.com/bio-routing/flowhouse/pkg/models/flow"
	"github.com/bio-routing/flowhouse/pkg/routemirror"
	"github.com/pkg/errors"
)

type IPAnnotator struct {
	rm *routemirror.RouteMirror
}

func New(rm *routemirror.RouteMirror) *IPAnnotator {
	return &IPAnnotator{
		rm: rm,
	}
}

func (ipa *IPAnnotator) Annotate(fl *flow.Flow) error {
	srt, err := ipa.rm.LPM(fl.Agent.String(), fl.VRFIn, fl.SrcAddr)
	if err != nil {
		return errors.Wrap(err, "Unable to get route for source address")
	}

	if srt == nil {
		return fmt.Errorf("No route found for %s", fl.SrcAddr.String())
	}

	fl.SrcPfx = *srt.Prefix()
	srcFirstASPathSeg := srt.BestPath().BGPPath.ASPath.GetFirstSequenceSegment()
	if srcFirstASPathSeg != nil {
		srcASN := srcFirstASPathSeg.GetFirstASN()
		if srcASN != nil {
			fl.SrcAs = *srcASN
		}
	}

	drt, err := ipa.rm.LPM(fl.Agent.String(), fl.VRFOut, fl.DstAddr)
	if err != nil {
		return errors.Wrap(err, "Unable to get route for source address")
	}

	if drt == nil {
		return fmt.Errorf("No route found for %s", fl.DstAddr.String())
	}

	fl.DstPfx = *drt.Prefix()
	dstLastASPathSeg := drt.BestPath().BGPPath.ASPath.GetLastSequenceSegment()
	if dstLastASPathSeg != nil {
		dstASN := dstLastASPathSeg.GetLastASN()
		if dstASN != nil {
			fl.DstAs = *dstASN
		}
	}

	dstFirstASPathSeg := drt.BestPath().BGPPath.ASPath.GetFirstSequenceSegment()
	if dstFirstASPathSeg != nil {
		nextASN := dstFirstASPathSeg.GetFirstASN()
		if nextASN != nil {
			fl.NextAs = *nextASN
		}
	}

	return nil
}
